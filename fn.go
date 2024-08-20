package main

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"text/template"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	fnv1beta1 "github.com/crossplane/function-sdk-go/proto/v1beta1"
	"github.com/crossplane/function-sdk-go/request"
	"github.com/crossplane/function-sdk-go/response"
	"github.com/crossplane/function-status-transformer/input/v1beta1"
)

type contextKey string

const (
	// Condition types.
	typeFunctionSuccess = "StatusTransformationSuccess"

	// Condition reasons.
	reasonAvailable                = "Available"
	reasonInputFailure             = "InputFailure"
	reasonObservedCompositeFailure = "ObservedCompositeFailure"
	reasonMatchFailure             = "MatchFailure"
	reasonSetConditionFailure      = "SetConditionFailure"

	// Context keys.
	logKey contextKey = "log"
)

// Function returns whatever response you ask it to.
type Function struct {
	fnv1beta1.UnimplementedFunctionRunnerServiceServer

	log logging.Logger
}

// RunFunction runs the Function.
func (f *Function) RunFunction(ctx context.Context, req *fnv1beta1.RunFunctionRequest) (*fnv1beta1.RunFunctionResponse, error) {
	log := f.log.WithValues("tag", req.GetMeta().GetTag())
	log.Debug("running function")

	rsp := response.To(req, response.DefaultTTL)

	in := &v1beta1.StatusTransformation{}
	if err := request.GetInput(req, in); err != nil {
		msg := fmt.Sprintf("cannot get Function input from %T", req)
		log.Info(msg, "error", err)
		response.ConditionFalse(rsp, typeFunctionSuccess, reasonInputFailure).
			WithMessage(errors.Wrap(err, msg).Error())
		return rsp, nil
	}

	xr, err := request.GetObservedCompositeResource(req)
	if err != nil {
		msg := fmt.Sprintf("cannot get observed XR from %T", req)
		log.Info(msg, "error", err)
		response.ConditionFalse(rsp, typeFunctionSuccess, reasonInputFailure).
			WithMessage(errors.Wrap(err, msg).Error())
		return rsp, nil
	}
	log = log.WithValues(
		"xr-apiversion", xr.Resource.GetAPIVersion(),
		"xr-kind", xr.Resource.GetKind(),
		"xr-name", xr.Resource.GetName(),
	)
	log.Info("running function")

	observed := map[string]*fnv1beta1.Resource{}
	if req.GetObserved() != nil && req.GetObserved().GetResources() != nil {
		observed = req.GetObserved().GetResources()
	}

	errored := false
	conditionsSet := map[string]bool{}
	for shi, sh := range in.StatusConditionHooks {
		log := log.WithValues("statusConditionHookIndex", shi)
		// The regular expression groups found in the matches.
		scGroups := map[string]string{}
		allMatched := false
		for mci, mc := range sh.Matchers {
			log := log.WithValues("matchConditionIndex", mci)
			ctx := context.WithValue(ctx, logKey, log)

			matched, mcGroups, err := matchResources(ctx, mc, observed)
			if err != nil {
				log.Info("cannot match resources", "error", err)
				response.ConditionFalse(rsp, typeFunctionSuccess, reasonMatchFailure).
					WithMessage(errors.Wrapf(err, "cannot match resources, statusConditionHookIndex: %d, matchConditionIndex: %d", shi, mci).Error())
				matched = false
				errored = true
			}

			if !matched {
				// All matchConditions must match.
				allMatched = false
				break
			}
			allMatched = true

			// All matches were successful, copy over any regex groups.
			for k, v := range mcGroups {
				scGroups[k] = v
			}
		}

		if !allMatched {
			// This hook did not match; do not set conditions.
			continue
		}

		// All matchConditions matched, set the desired conditions.
		for sci, cs := range sh.SetConditions {
			log := log.WithValues("setConditionIndex", sci)
			if conditionsSet[cs.Condition.Type] && (cs.Force == nil || !*cs.Force) {
				// The condition is already set and this setter is not forceful.
				log.Debug("skipping because condition is already set and setCondition is not forceful")
				continue
			}
			log.Debug("setting condition")

			c, err := transformCondition(cs, scGroups)
			if err != nil {
				log.Info("cannot set condition", "error", err)
				response.ConditionFalse(rsp, typeFunctionSuccess, reasonSetConditionFailure).
					WithMessage(errors.Wrapf(err, "cannot set condition, statusConditionHookIndex: %d, setConditionIndex: %d", shi, sci).Error())
				errored = true
				continue
			}

			rsp.Conditions = append(rsp.Conditions, c)
			conditionsSet[cs.Condition.Type] = true
		}

		for cei, ce := range sh.CreateEvents {
			log := log.WithValues("createEventIndex", cei)
			r, err := transformEvent(ce, scGroups)
			if err != nil {
				log.Info("cannot create event")
				response.ConditionFalse(rsp, typeFunctionSuccess, reasonSetConditionFailure).
					WithMessage(errors.Wrapf(err, "cannot create event, statusConditionHookIndex: %d, createEventIndex: %d", shi, cei).Error())
				errored = true
				continue
			}

			rsp.Results = append(rsp.Results, r)
		}
	}

	if !errored {
		response.ConditionTrue(rsp, typeFunctionSuccess, reasonAvailable)
	}

	return rsp, nil
}

func matchResources(ctx context.Context, mc v1beta1.Matcher, observedMap map[string]*fnv1beta1.Resource) (bool, map[string]string, error) {
	log := ctx.Value(logKey).(logging.Logger)

	rs := map[string]*fnv1beta1.Resource{}
	for i, r := range mc.Resources {
		re, err := regexp.Compile(r.Name)
		if err != nil {
			log.Info("cannot compile resource key regex", "resourcesIndex", i, "error", err)
			return false, nil, errors.Wrapf(err, "cannot compile resource key regex, resourcesIndex: %d", i)
		}
		for k, v := range observedMap {
			if re.MatchString(k) {
				rs[k] = v
			}
		}
	}

	if len(rs) == 0 {
		// There are no resources to match against.
		return false, nil, nil
	}
	if len(mc.Conditions) == 0 {
		// There are no conditions to match against.
		return false, nil, nil
	}

	switch ptr.Deref(mc.Type, v1beta1.AllResourcesMatchAllConditions) {
	case v1beta1.AnyResourceMatchesAnyCondition:
		return anyResourceMatchesAnyCondition(ctx, mc.Conditions, rs)
	case v1beta1.AnyResourceMatchesAllConditions:
		return anyResourceMatchesAllConditions(ctx, mc.Conditions, rs)
	case v1beta1.AllResourcesMatchAnyCondition:
		return allResourcesMatchAnyConditions(ctx, mc.Conditions, rs)
	case v1beta1.AllResourcesMatchAllConditions:
		fallthrough
	default:
		return allResourcesMatchAllConditions(ctx, mc.Conditions, rs)
	}
}

func anyResourceMatchesAnyCondition(ctx context.Context, cms []v1beta1.ConditionMatcher, rm map[string]*fnv1beta1.Resource) (bool, map[string]string, error) {
	log := ctx.Value(logKey).(logging.Logger)
	for k, r := range rm {
		for cmi, cm := range cms {
			log := log.WithValues("resource", k, "conditionIndex", cmi)
			ctx := context.WithValue(ctx, logKey, log)
			m, cg, err := match(ctx, cm, r)
			if err != nil {
				log.Info("cannot match resource", "error", err)
				return false, nil, err
			}

			if m {
				return true, cg, nil
			}
		}
	}

	return false, nil, nil
}

func anyResourceMatchesAllConditions(ctx context.Context, cms []v1beta1.ConditionMatcher, rm map[string]*fnv1beta1.Resource) (bool, map[string]string, error) {
	log := ctx.Value(logKey).(logging.Logger)
	capturedGroups := map[string]string{}
	for k, r := range rm {
		matched := 0
		for cmi, cm := range cms {
			log := log.WithValues("resource", k, "conditionIndex", cmi)
			ctx := context.WithValue(ctx, logKey, log)
			m, cg, err := match(ctx, cm, r)
			if err != nil {
				log.Info("cannot match resource", "error", err)
				return false, nil, err
			}
			if !m {
				break
			}
			matched++
			for k, v := range cg {
				capturedGroups[k] = v
			}
		}
		if matched == len(cms) {
			return true, capturedGroups, nil
		}
	}

	return false, nil, nil
}

func allResourcesMatchAnyConditions(ctx context.Context, cms []v1beta1.ConditionMatcher, rm map[string]*fnv1beta1.Resource) (bool, map[string]string, error) {
	log := ctx.Value(logKey).(logging.Logger)
	capturedGroups := map[string]string{}
	for k, r := range rm {
		matched := 0
		for cmi, cm := range cms {
			log := log.WithValues("resource", k, "conditionIndex", cmi)
			ctx := context.WithValue(ctx, logKey, log)
			m, cg, err := match(ctx, cm, r)
			if err != nil {
				log.Info("cannot match resource", "error", err)
				return false, nil, err
			}
			if !m {
				continue
			}
			matched++
			for k, v := range cg {
				capturedGroups[k] = v
			}
		}
		if matched == 0 {
			return false, nil, nil
		}
	}

	return true, capturedGroups, nil
}

func allResourcesMatchAllConditions(ctx context.Context, cms []v1beta1.ConditionMatcher, rm map[string]*fnv1beta1.Resource) (bool, map[string]string, error) {
	log := ctx.Value(logKey).(logging.Logger)
	capturedGroups := map[string]string{}
	for k, r := range rm {
		for cmi, cm := range cms {
			log := log.WithValues("resource", k, "conditionIndex", cmi)
			ctx := context.WithValue(ctx, logKey, log)
			m, cg, err := match(ctx, cm, r)
			if err != nil {
				log.Info("cannot match resource", "error", err)
				return false, nil, err
			}
			if !m {
				return false, nil, nil
			}
			for k, v := range cg {
				capturedGroups[k] = v
			}
		}
	}

	return true, capturedGroups, nil
}

func match(ctx context.Context, cm v1beta1.ConditionMatcher, r *fnv1beta1.Resource) (bool, map[string]string, error) {
	log := ctx.Value(logKey).(logging.Logger)
	cmGroups := map[string]string{}
	conditioned := xpv1.ConditionedStatus{}

	// Ignoring error. If field is missing, we will default to unknown.
	_ = fieldpath.Pave(r.GetResource().AsMap()).GetValueInto("status", &conditioned)
	c := conditioned.GetCondition(xpv1.ConditionType(cm.Type))
	if cm.Reason != nil && *cm.Reason != string(c.Reason) {
		log.Debug(fmt.Sprintf("condition reason \"%s\" did not match \"%s\"", c.Reason, *cm.Reason))
		return false, nil, nil
	}

	if cm.Status != nil && *cm.Status != v1.ConditionStatus(c.Status) {
		log.Debug(fmt.Sprintf("condition status \"%s\" did not match \"%s\"", c.Status, *cm.Status))
		return false, nil, nil
	}

	if cm.Message == nil {
		log.Debug("condition matched")
		return true, nil, nil
	}

	// match message and build up map of args
	re, err := regexp.Compile(*cm.Message)
	if err != nil {
		return false, nil, errors.Wrap(err, "cannot compile message regex")
	}

	matches := re.FindStringSubmatch(c.Message)
	if len(matches) == 0 {
		log.Debug(fmt.Sprintf("condition message \"%s\" did not match \"%s\"", c.Message, *cm.Message))
		return false, nil, nil
	}

	for i := 1; i < len(matches); i++ {
		cmGroups[re.SubexpNames()[i]] = matches[i]
	}
	log.Debug(fmt.Sprintf("condition matched - total captured groups: %v", cmGroups))

	return true, cmGroups, nil
}

func transformCondition(cs v1beta1.SetCondition, templateValues map[string]string) (*fnv1beta1.Condition, error) {
	c := &fnv1beta1.Condition{
		Type:   cs.Condition.Type,
		Reason: cs.Condition.Reason,
		Target: transformTarget(cs.Target),
	}

	switch cs.Condition.Status {
	case v1.ConditionTrue:
		c.Status = fnv1beta1.Status_STATUS_CONDITION_TRUE
	case v1.ConditionFalse:
		c.Status = fnv1beta1.Status_STATUS_CONDITION_FALSE
	case v1.ConditionUnknown:
		fallthrough
	default:
		c.Status = fnv1beta1.Status_STATUS_CONDITION_UNKNOWN
	}

	msg, err := templateMessage(cs.Condition.Message, templateValues)
	if err != nil {
		return &fnv1beta1.Condition{}, err
	}
	c.Message = msg

	return c, nil
}

func transformEvent(ec v1beta1.CreateEvent, templateValues map[string]string) (*fnv1beta1.Result, error) {
	e := &fnv1beta1.Result{
		Reason: ec.Event.Reason,
		Target: transformTarget(ec.Target),
	}

	switch ptr.Deref(ec.Event.Type, v1beta1.EventTypeNormal) {
	case v1beta1.EventTypeNormal:
		e.Severity = fnv1beta1.Severity_SEVERITY_NORMAL
	case v1beta1.EventTypeWarning:
		e.Severity = fnv1beta1.Severity_SEVERITY_WARNING
	default:
		return &fnv1beta1.Result{}, errors.Errorf("invalid type %s, must be one of [Normal, Warning]", *ec.Event.Type)
	}

	msg, err := templateMessage(&ec.Event.Message, templateValues)
	if err != nil {
		return &fnv1beta1.Result{}, err
	}
	e.Message = ptr.Deref(msg, "")
	return e, nil
}

func transformTarget(t *v1beta1.Target) *fnv1beta1.Target {
	target := ptr.Deref(t, v1beta1.TargetComposite)
	if target == v1beta1.TargetCompositeAndClaim {
		return fnv1beta1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum()
	}
	return fnv1beta1.Target_TARGET_COMPOSITE.Enum()
}

func templateMessage(msg *string, values map[string]string) (*string, error) {
	if msg == nil || len(values) == 0 {
		return msg, nil
	}

	t, err := template.New("").Parse(*msg)
	if err != nil {
		return nil, errors.Wrap(err, "cannot parse template")
	}
	b := bytes.NewBuffer(nil)
	if err := t.Execute(b, values); err != nil {
		return nil, errors.Wrap(err, "cannot execute template")
	}
	return ptr.To(b.String()), nil
}
