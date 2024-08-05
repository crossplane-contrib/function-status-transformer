// Package v1beta1 contains the input type for this Function
// +kubebuilder:object:generate=true
// +groupName=function-status-transformer.fn.crossplane.io
// +versionName=v1beta1
package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StatusTransformation can be used to provide input to this Function.
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:resource:categories=crossplane
type StatusTransformation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	StatusConditionHooks []StatusConditionHook `json:"statusConditionHooks"`
}

// Target determines which objects to set the condition on.
type Target string

const (
	// TargetComposite targets only the composite resource.
	TargetComposite Target = "Composite"

	// TargetCompositeAndClaim targets both the composite and the claim.
	TargetCompositeAndClaim Target = "CompositeAndClaim"
)

// +kubebuilder:validation:Enum=MatchAny;MatchAll

// MatchType determines matching behavior.
type MatchType string

const (
	// MatchAll resources.
	MatchAll MatchType = "MatchAll"

	// MatchAny resource.
	MatchAny MatchType = "MatchAny"
)

// SetCondition will set a condition on the target.
type SetCondition struct {
	// The target(s) to receive the condition. Can be Composite or
	// CompositeAndClaim.
	Target *Target `json:"target"`
	// If true, the condition will override a condition of the same Type. Defaults
	// to false.
	Force *bool `json:"force"`
	// Condition to set.
	Condition Condition `json:"condition"`
}

// Condition allows you to specify fields to set on a composite resource and
// claim.
type Condition struct {
	// Type of the condition. Required.
	Type string `json:"type"`
	// Status of the condition. Required.
	Status metav1.ConditionStatus `json:"status"`
	// Reason of the condition. Required.
	Reason string `json:"reason"`
	// Message of the condition. Optional. A template can be used. The available
	// template variables come from capturing groups in MatchCondition message
	// regular expressions.
	Message *string `json:"message"`
}

// MatchCondition will attempt to match a condition on the resource.
type MatchCondition struct {
	// The name of the resource to match against or a regex to match multiple
	// resources. This is matching against the keys used in the observed and
	// desired resource maps.
	ResourceName string `json:"resourceName"`

	// Will determine the behavior if matching multiple resources by using a
	// regular expression as your ResourceName. Can be MatchAll or MatchAny.
	// MatchAll requires all resources to match the condition. MatchAny requires
	// any of the resources to match the condition.
	Type *MatchType `json:"type"`

	// Condition that must exist on the resource(s).
	Condition ConditionMatch `json:"condition"`
}

// ConditionMatch allows you to specify fields that a condition must match.
type ConditionMatch struct {
	// Type of the condition. Required.
	Type string `json:"type"`
	// Status of the condition. If omitted, will be treated as a wildcard.
	Status *metav1.ConditionStatus `json:"status"`
	// Reason of the condition. If omitted, will be treated as a wildcard.
	Reason *string `json:"reason"`
	// Message of the condition. Can be a regular expression. The regular
	// expression can have capturing groups.
	// For example: "Something went wrong: (?P<Error>.+)".
	// The captured groups will be available to the message template when setting
	// conditions.
	Message *string `json:"message"`
}

// StatusConditionHook allows you to set conditions on the composite and claim
// whenever the managed resource status conditions are in a certain state.
type StatusConditionHook struct {
	// A list of conditions to match.
	MatchConditions []MatchCondition `json:"matchConditions"`

	// A list of conditions to set if all MatchConditions matched.
	SetConditions []SetCondition `json:"setConditions"`

	// A list of events to create if all MatchConditions matched.
	CreateEvents []CreateEvent `json:"createEvents"`
}

// EventType type of an event.
type EventType string

const (
	// EventTypeNormal signifies a normal event.
	EventTypeNormal EventType = "Normal"

	// EventTypeWarning signifies a warning event.
	EventTypeWarning EventType = "Warning"
)

// Event allows you to specify the fields of an event to create.
type Event struct {
	// Type of the event. Optional. Should be either Normal or Warning.
	Type *EventType `json:"type"`
	// Reason of the event. Optional.
	Reason *string `json:"reason"`
	// Message of the event. Required. A template can be used. The available
	// template variables come from capturing groups in MatchCondition message
	// regular expressions.
	Message string `json:"message"`
}

// CreateEvent will create an event for the target(s).
type CreateEvent struct {
	// The target(s) to create an event for. Can be Composite or
	// CompositeAndClaim.
	Target *Target `json:"target"`

	// Event to create.
	Event Event `json:"event"`
}
