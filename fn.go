package main

import (
	"bytes"
	"context"
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

const (
	// Condition types.
	typeFunctionSuccess = "StatusTransformationSuccess"

	// Condition reasons.
	reasonAvailable                = "Available"
	reasonInputFailure             = "InputFailure"
	reasonObservedCompositeFailure = "ObservedCompositeFailure"
	reasonMatchFailure             = "MatchFailure"
	reasonSetConditionFailure      = "SetConditionFailure"
)

// Function returns whatever response you ask it to.
type Function struct {
	fnv1beta1.UnimplementedFunctionRunnerServiceServer

	log logging.Logger
}

// RunFunction runs the Function.
func (f *Function) RunFunction(_ context.Context, req *fnv1beta1.RunFunctionRequest) (*fnv1beta1.RunFunctionResponse, error) {
	log := f.log.WithValues("tag", req.GetMeta().GetTag())
	log.Debug("running function")

	rsp := response.To(req, response.DefaultTTL)

	in := &v1beta1.StatusTransformation{}
	if err := request.GetInput(req, in); err != nil {
		msg := errors.Wrapf(err, "cannot get Function input from %T", req).Error()
		log.Info(msg)
		response.ConditionFalse(rsp, typeFunctionSuccess, reasonInputFailure).
			WithMessage(msg)
		return rsp, nil
	}

	xr, err := request.GetObservedCompositeResource(req)
	if err != nil {
		msg := errors.Wrapf(err, "cannot get observed XR from %T", req).Error()
		log.Info(msg)
		response.ConditionFalse(rsp, typeFunctionSuccess, reasonInputFailure).
			WithMessage(msg)
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
		// The regular expression groups found in the matches.
		scGroups := map[string]string{}
		shMatched := true
		for mci, mc := range sh.MatchConditions {
			mcMatched, mcGroups, err := matchConditions(mc, observed)
			if err != nil {
				log.Info("error when matching", "error", err, "statusConditionHookIndex", shi, "matchConditionIndex", mci)
				response.ConditionFalse(rsp, typeFunctionSuccess, reasonMatchFailure).
					WithMessage(errors.Wrapf(err, "error when matching, statusConditionHookIndex: %d, matchConditionIndex: %d", shi, mci).Error())
				mcMatched = false
				errored = true
			}
			if !mcMatched {
				shMatched = false
				break
			}
			// All matches were successful, copy over any regex groups.
			for k, v := range mcGroups {
				scGroups[k] = v
			}
		}

		if !shMatched {
			// This hook did not match; do not set conditions.
			continue
		}

		// All matchConditions matched, set the desired conditions.
		for sci, cs := range sh.SetConditions {
			if conditionsSet[cs.Condition.Type] && (cs.Force == nil || !*cs.Force) {
				// The condition is already set and this setter is not forceful.
				continue
			}

			c, err := transformCondition(cs, scGroups)
			if err != nil {
				msg := errors.Wrapf(err, "failed to set condition, statusConditionHookIndex: %d, setConditionIndex: %d", shi, sci).Error()
				log.Info(msg)
				response.ConditionFalse(rsp, typeFunctionSuccess, reasonSetConditionFailure).
					WithMessage(msg)
				errored = true
				continue
			}

			rsp.Conditions = append(rsp.Conditions, c)
			conditionsSet[cs.Condition.Type] = true
		}

		for cei, ce := range sh.CreateEvents {
			r, err := transformEvent(ce, scGroups)
			if err != nil {
				msg := errors.Wrapf(err, "failed to create event, statusConditionHookIndex: %d, createEventIndex: %d", shi, cei).Error()
				log.Info(msg)
				response.ConditionFalse(rsp, typeFunctionSuccess, reasonSetConditionFailure).
					WithMessage(msg)
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

func matchConditions(cm v1beta1.MatchCondition, om map[string]*fnv1beta1.Resource) (bool, map[string]string, error) {
	re, err := regexp.Compile(cm.ResourceName)
	if err != nil {
		return false, nil, errors.Join(errors.New("failed to compile resourceName regex"), err)
	}
	cmGroups := map[string]string{}
	matchedAny := false
	matchedAll := true
	for k, o := range om {
		if !re.MatchString(k) {
			continue
		}
		// for each observed object with a resource name matching the regex,
		// check if the status condition matches the match condition
		conditioned := xpv1.ConditionedStatus{}
		// Ignoring error. If field is missing, we will default to unknown.
		_ = fieldpath.Pave(o.GetResource().AsMap()).GetValueInto("status", &conditioned)
		c := conditioned.GetCondition(xpv1.ConditionType(cm.Condition.Type))
		if cm.Condition.Reason != nil && *cm.Condition.Reason != string(c.Reason) {
			matchedAll = false
			continue
		}

		if cm.Condition.Status != nil && *cm.Condition.Status != v1.ConditionStatus(c.Status) {
			matchedAll = false
			continue
		}

		if cm.Condition.Message == nil {
			matchedAny = true
			continue
		}

		// match message and build up map of args
		re, err := regexp.Compile(*cm.Condition.Message)
		if err != nil {
			return false, nil, errors.Join(errors.New("failed to compile message regex"), err)
		}

		matches := re.FindStringSubmatch(c.Message)
		if len(matches) == 0 {
			matchedAll = false
			continue
		}
		matchedAny = true

		for i := 1; i < len(matches); i++ {
			cmGroups[re.SubexpNames()[i]] = matches[i]
		}
	}

	var matched bool
	switch ptr.Deref(cm.Type, v1beta1.MatchAll) {
	case v1beta1.MatchAll:
		matched = matchedAll
	case v1beta1.MatchAny:
		matched = matchedAny
	}

	return matched, cmGroups, nil
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
		return nil, errors.Join(errors.New("failed to parse template"), err)
	}
	b := bytes.NewBuffer(nil)
	if err := t.Execute(b, values); err != nil {
		return nil, errors.Join(errors.New("failed to execute template"), err)
	}
	return ptr.To(b.String()), nil
}
