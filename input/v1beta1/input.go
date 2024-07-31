// Package v1beta1 contains the input type for this Function
// +kubebuilder:object:generate=true
// +groupName=managed-resource-hook.fn.crossplane.io
// +versionName=v1beta1
package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ManagedResourceHook can be used to provide input to this Function.
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:resource:categories=crossplane
type ManagedResourceHook struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	StatusConditionHooks []StatusConditionHook `json:"statusConditionHooks"`
}

// ConditionTarget determines which objects to set the condition on.
type ConditionTarget string

const (
	// TargetComposite targets only the composite resource.
	TargetComposite ConditionTarget = "Composite"

	// TargetCompositeAndClaim targets both the composite and the claim.
	TargetCompositeAndClaim ConditionTarget = "CompositeAndClaim"
)

// ConditionSetter will set a condition on the target.
type ConditionSetter struct {
	Type    string                 `json:"type"`
	Status  metav1.ConditionStatus `json:"status"`
	Reason  string                 `json:"reason"`
	Message *string                `json:"message"`
	Target  ConditionTarget        `json:"target"`
	Force   *bool                  `json:"force"`
}

// ConditionMatcher will attempt to match a condition on the resource.
type ConditionMatcher struct {
	ResourceName string                  `json:"resourceName"`
	Type         string                  `json:"type"`
	Status       *metav1.ConditionStatus `json:"status"`
	Reason       *string                 `json:"reason"`
	Message      *string                 `json:"message"`
}

// StatusConditionHook allows you to set conditions on the composite and claim
// whenever the managed resource status conditions are in a certain state.
type StatusConditionHook struct {
	// A list of conditions to match.
	MatchConditions []ConditionMatcher `json:"matchConditions"`

	// A list of conditions to set if all MatchConditions matched.
	SetConditions []ConditionSetter `json:"setConditions"`
}
