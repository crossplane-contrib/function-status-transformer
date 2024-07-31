package main

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"
	"k8s.io/utils/ptr"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	fnv1beta1 "github.com/crossplane/function-sdk-go/proto/v1beta1"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/response"
)

func TestRunFunction(t *testing.T) {

	type args struct {
		ctx context.Context
		req *fnv1beta1.RunFunctionRequest
	}
	type want struct {
		rsp *fnv1beta1.RunFunctionResponse
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"CapturesRegexGroups": {
			reason: "The function should be able to capture regex groups and use them when setting conditions.",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`
{
  "apiVersion": "managed-resource-hook.fn.crossplane.io/v1beta1",
  "kind": "ManagedResourceHook",
  "statusConditionHooks": [
    {
      "matchConditions": [
        {
          "resourceName": "example-mr",
          "type": "Synced",
          "status": "False",
          "reason": "ReconcileError",
					"message": "Something went wrong: (?P<Error>.+)"
        }
      ],
      "setConditions": [
        {
          "type": "CustomReady",
          "status": "False",
          "reason": "InternalError",
          "message": "{{ .Error }}",
          "target": "Composite"
        }
      ]
    }
  ]
}
`),
					Observed: &fnv1beta1.State{
						Resources: map[string]*fnv1beta1.Resource{
							"example-mr": {
								Resource: resource.MustStructJSON(`
{
    "apiVersion": "some.example.com/v1alpha1",
    "kind": "Object",
    "metadata": {
      "name": "example-name"
    },
    "status": {
      "conditions": [
        {
					"message": "Something went wrong: some lower level error",
          "reason": "ReconcileError",
          "status": "False",
          "type": "Synced"
        }
      ]
    }
  }`),
							},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta:    &fnv1beta1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{},
					Conditions: []*fnv1beta1.Condition{
						{
							Type:   "CustomReady",
							Status: fnv1beta1.Status_STATUS_CONDITION_FALSE,
							Reason: "InternalError",
							// We should only see the lower level error included.
							Message: ptr.To("some lower level error"),
							Target:  fnv1beta1.Target_TARGET_COMPOSITE.Enum(),
						},
						{
							Type:   "StatusConditionsReady",
							Status: fnv1beta1.Status_STATUS_CONDITION_TRUE,
							Reason: "Available",
							Target: fnv1beta1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
				},
			},
		},
		"WildcardMatching": {
			reason: "When a matchCondition field is nil, it should act as a wildcard.",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`
{
  "apiVersion": "managed-resource-hook.fn.crossplane.io/v1beta1",
  "kind": "ManagedResourceHook",
  "statusConditionHooks": [
    {
      "matchConditions": [
        {
          "resourceName": "example-mr",
          "type": "Synced",
          "reason": "ReconcileError",
					"message": "Something went wrong."
        }
      ],
      "setConditions": [
        {
          "type": "NilStatus",
          "status": "True",
          "reason": "Test",
					"message": "Testing wildcard matching.",
          "target": "Composite"
        }
      ]
    },
    {
      "matchConditions": [
        {
          "resourceName": "example-mr",
          "type": "Synced",
					"status": "False",
					"message": "Something went wrong."
        }
      ],
      "setConditions": [
        {
          "type": "NilReason",
          "status": "True",
          "reason": "Test",
					"message": "Testing wildcard matching.",
          "target": "Composite"
        }
      ]
    },
    {
      "matchConditions": [
        {
          "resourceName": "example-mr",
          "type": "Synced",
					"status": "False",
          "reason": "ReconcileError"
        }
      ],
      "setConditions": [
        {
          "type": "NilMessage",
          "status": "True",
          "reason": "Test",
					"message": "Testing wildcard matching.",
          "target": "Composite"
        }
      ]
    },
    {
      "matchConditions": [
        {
          "resourceName": "example-mr",
          "type": "Synced"
        }
      ],
      "setConditions": [
        {
          "type": "NilAll",
          "status": "True",
          "reason": "Test",
					"message": "Testing wildcard matching.",
          "target": "Composite"
        }
      ]
    }
  ]
}
`),
					Observed: &fnv1beta1.State{
						Resources: map[string]*fnv1beta1.Resource{
							"example-mr": {
								Resource: resource.MustStructJSON(`
{
    "apiVersion": "some.example.com/v1alpha1",
    "kind": "Object",
    "metadata": {
      "name": "example-name"
    },
    "status": {
      "conditions": [
        {
					"message": "Something went wrong.",
          "reason": "ReconcileError",
          "status": "False",
          "type": "Synced"
        }
      ]
    }
  }`),
							},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta:    &fnv1beta1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{},
					Conditions: []*fnv1beta1.Condition{
						{
							Type:    "NilStatus",
							Status:  fnv1beta1.Status_STATUS_CONDITION_TRUE,
							Reason:  "Test",
							Message: ptr.To("Testing wildcard matching."),
							Target:  fnv1beta1.Target_TARGET_COMPOSITE.Enum(),
						},
						{
							Type:    "NilReason",
							Status:  fnv1beta1.Status_STATUS_CONDITION_TRUE,
							Reason:  "Test",
							Message: ptr.To("Testing wildcard matching."),
							Target:  fnv1beta1.Target_TARGET_COMPOSITE.Enum(),
						},
						{
							Type:    "NilMessage",
							Status:  fnv1beta1.Status_STATUS_CONDITION_TRUE,
							Reason:  "Test",
							Message: ptr.To("Testing wildcard matching."),
							Target:  fnv1beta1.Target_TARGET_COMPOSITE.Enum(),
						},
						{
							Type:    "NilAll",
							Status:  fnv1beta1.Status_STATUS_CONDITION_TRUE,
							Reason:  "Test",
							Message: ptr.To("Testing wildcard matching."),
							Target:  fnv1beta1.Target_TARGET_COMPOSITE.Enum(),
						},
						{
							Type:   "StatusConditionsReady",
							Status: fnv1beta1.Status_STATUS_CONDITION_TRUE,
							Reason: "Available",
							Target: fnv1beta1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
				},
			},
		},
		"MatchConditionsAnded": {
			reason: "Match conditions should be ANDed together.",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`
{
  "apiVersion": "managed-resource-hook.fn.crossplane.io/v1beta1",
  "kind": "ManagedResourceHook",
  "statusConditionHooks": [
    {
      "matchConditions": [
        {
          "resourceName": "example-mr",
          "type": "Synced",
          "status": "False",
          "reason": "ReconcileError"
        },
        {
          "resourceName": "example-mr",
          "type": "Ready",
          "status": "False",
          "reason": "ReconcileError"
        }
      ],
      "setConditions": [
        {
          "type": "MatchedBoth",
          "status": "True",
          "reason": "ShouldMatchBoth",
          "message": "Match conditions are ANDed together. Both conditions should match.",
          "target": "Composite"
        }
      ]
    },
		{
      "matchConditions": [
        {
          "resourceName": "example-mr",
          "type": "Synced",
          "status": "False",
          "reason": "ReconcileError"
        },
        {
          "resourceName": "example-mr",
          "type": "DoesNotExist",
          "status": "False",
          "reason": "ReconcileError"
        }
      ],
      "setConditions": [
        {
          "type": "MatchedOne",
          "status": "True",
          "reason": "OneDidNotMatch",
          "message": "Should not match on the DoesNotExist condition.",
          "target": "Composite"
        }
      ]
    }
  ]
}
`),
					Observed: &fnv1beta1.State{
						Resources: map[string]*fnv1beta1.Resource{
							"example-mr": {
								Resource: resource.MustStructJSON(`
{
    "apiVersion": "some.example.com/v1alpha1",
    "kind": "Object",
    "metadata": {
      "name": "example-name"
    },
    "status": {
      "conditions": [
        {
          "type": "Synced",
          "reason": "ReconcileError",
          "status": "False"
        },
        {
          "type": "Ready",
          "reason": "ReconcileError",
          "status": "False"
        }
      ]
    }
  }`),
							},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta:    &fnv1beta1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{},
					Conditions: []*fnv1beta1.Condition{
						{
							Type:    "MatchedBoth",
							Status:  fnv1beta1.Status_STATUS_CONDITION_TRUE,
							Reason:  "ShouldMatchBoth",
							Message: ptr.To("Match conditions are ANDed together. Both conditions should match."),
							Target:  fnv1beta1.Target_TARGET_COMPOSITE.Enum(),
						},
						{
							Type:   "StatusConditionsReady",
							Status: fnv1beta1.Status_STATUS_CONDITION_TRUE,
							Reason: "Available",
							Target: fnv1beta1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
				},
			},
		},
		"ConditionOverrideLogic": {
			reason: "The same condition should not be overridden by default. Override should be possible by setting force.",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`
{
  "apiVersion": "managed-resource-hook.fn.crossplane.io/v1beta1",
  "kind": "ManagedResourceHook",
  "statusConditionHooks": [
    {
      "matchConditions": [
        {
          "resourceName": "example-mr",
          "type": "Synced",
          "status": "False",
          "reason": "ReconcileError"
        }
      ],
      "setConditions": [
        {
          "type": "ConditionA",
          "status": "True",
          "reason": "SetFirst",
          "target": "CompositeAndClaim"
        },
        {
          "type": "ConditionA",
          "status": "True",
          "reason": "SetSecond",
					"message": "Should not be set as force is not used.",
          "target": "CompositeAndClaim"
        },
				{
          "type": "ConditionB",
          "status": "True",
          "reason": "SetFirst",
          "target": "CompositeAndClaim"
        },
				{
          "type": "ConditionB",
          "status": "True",
          "reason": "SetSecond",
          "target": "CompositeAndClaim",
					"force": true
        }
      ]
    },
    {
      "matchConditions": [
        {
          "resourceName": "example-mr",
          "type": "Synced",
          "status": "False",
          "reason": "ReconcileError"
        }
      ],
      "setConditions": [
        {
          "type": "ConditionA",
          "status": "True",
          "reason": "SetSecondSeparateMatch",
					"message": "Should not be set as force is not used.",
          "target": "CompositeAndClaim"
        },
				{
          "type": "ConditionB",
          "status": "True",
          "reason": "SetThird",
          "target": "CompositeAndClaim",
					"force": true
        }
      ]
    }
  ]
}
`),
					Observed: &fnv1beta1.State{
						Resources: map[string]*fnv1beta1.Resource{
							"example-mr": {
								Resource: resource.MustStructJSON(`
{
    "apiVersion": "some.example.com/v1alpha1",
    "kind": "Object",
    "metadata": {
      "name": "example-name"
    },
    "status": {
      "conditions": [
        {
					"message": "Something went wrong: some lower level error",
          "reason": "ReconcileError",
          "status": "False",
          "type": "Synced"
        }
      ]
    }
  }`),
							},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta:    &fnv1beta1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{},
					Conditions: []*fnv1beta1.Condition{
						{
							Type:   "ConditionA",
							Status: fnv1beta1.Status_STATUS_CONDITION_TRUE,
							Reason: "SetFirst",
							Target: fnv1beta1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
						{
							Type:   "ConditionB",
							Status: fnv1beta1.Status_STATUS_CONDITION_TRUE,
							Reason: "SetFirst",
							Target: fnv1beta1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
						{
							Type:   "ConditionB",
							Status: fnv1beta1.Status_STATUS_CONDITION_TRUE,
							Reason: "SetSecond",
							Target: fnv1beta1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
						{
							Type:   "ConditionB",
							Status: fnv1beta1.Status_STATUS_CONDITION_TRUE,
							Reason: "SetThird",
							Target: fnv1beta1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
						{
							Type:   "StatusConditionsReady",
							Status: fnv1beta1.Status_STATUS_CONDITION_TRUE,
							Reason: "Available",
							Target: fnv1beta1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
				},
			},
		},
		"HandlesMissingResources": {
			reason: "You should be able to set conditions for missing resources by matching on the default Unknown status.",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`
{
  "apiVersion": "managed-resource-hook.fn.crossplane.io/v1beta1",
  "kind": "ManagedResourceHook",
  "statusConditionHooks": [
    {
      "matchConditions": [
        {
          "resourceName": "example-mr",
          "type": "Synced",
          "status": "Unknown",
          "reason": "",
					"message": ""
        }
      ],
      "setConditions": [
        {
          "type": "CustomReady",
          "status": "False",
          "reason": "DoesNotExist",
          "target": "CompositeAndClaim"
        }
      ]
    }
  ]
}
`),
					Observed: &fnv1beta1.State{
						Resources: map[string]*fnv1beta1.Resource{},
					},
				},
			},
			want: want{
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta:    &fnv1beta1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{},
					Conditions: []*fnv1beta1.Condition{
						{
							Type:   "CustomReady",
							Status: fnv1beta1.Status_STATUS_CONDITION_FALSE,
							Reason: "DoesNotExist",
							Target: fnv1beta1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
						{
							Type:   "StatusConditionsReady",
							Status: fnv1beta1.Status_STATUS_CONDITION_TRUE,
							Reason: "Available",
							Target: fnv1beta1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
				},
			},
		},
		"MatchRegexFailure": {
			reason: "The function should set the shared status condition to false when encountering a regex failure when matching.",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`
{
  "apiVersion": "managed-resource-hook.fn.crossplane.io/v1beta1",
  "kind": "ManagedResourceHook",
  "statusConditionHooks": [
    {
      "matchConditions": [
        {
          "resourceName": "example-mr",
          "type": "Synced",
          "status": "False",
          "reason": "ReconcileError",
					"message": "a bad regex (?!)"
        }
      ],
      "setConditions": [
        {
          "type": "CustomReady",
          "status": "False",
          "reason": "InternalError",
          "message": "{{ .Error }}",
          "target": "Composite"
        }
      ]
    }
  ]
}
`),
					Observed: &fnv1beta1.State{
						Resources: map[string]*fnv1beta1.Resource{
							"example-mr": {
								Resource: resource.MustStructJSON(`
{
    "apiVersion": "some.example.com/v1alpha1",
    "kind": "Object",
    "metadata": {
      "name": "example-name"
    },
    "status": {
      "conditions": [
        {
					"message": "Something went wrong: some lower level error",
          "reason": "ReconcileError",
          "status": "False",
          "type": "Synced"
        }
      ]
    }
  }`),
							},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta:    &fnv1beta1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{},
					Conditions: []*fnv1beta1.Condition{
						{
							Type:    "StatusConditionsReady",
							Status:  fnv1beta1.Status_STATUS_CONDITION_FALSE,
							Reason:  "MatchFailure",
							Target:  fnv1beta1.Target_TARGET_COMPOSITE.Enum(),
							Message: ptr.To("error when matching, statusConditionHookIndex: 0, matchConditionIndex: 0: [failed to compile message regex, error parsing regexp: invalid or unsupported Perl syntax: `(?!`]"),
						},
					},
				},
			},
		},
		"TemplateParseFailure": {
			reason: "The function should set the shared status condition to false when encountering a template parsing error.",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`
{
  "apiVersion": "managed-resource-hook.fn.crossplane.io/v1beta1",
  "kind": "ManagedResourceHook",
  "statusConditionHooks": [
    {
      "matchConditions": [
        {
          "resourceName": "example-mr",
          "type": "Synced",
          "status": "False",
          "reason": "ReconcileError",
					"message": "Something went wrong: (?P<Error>.+)"
        }
      ],
      "setConditions": [
        {
          "type": "CustomReady",
          "status": "False",
          "reason": "InternalError",
          "message": "{{ .Error }",
          "target": "Composite"
        }
      ]
    }
  ]
}
`),
					Observed: &fnv1beta1.State{
						Resources: map[string]*fnv1beta1.Resource{
							"example-mr": {
								Resource: resource.MustStructJSON(`
{
    "apiVersion": "some.example.com/v1alpha1",
    "kind": "Object",
    "metadata": {
      "name": "example-name"
    },
    "status": {
      "conditions": [
        {
					"message": "Something went wrong: some lower level error",
          "reason": "ReconcileError",
          "status": "False",
          "type": "Synced"
        }
      ]
    }
  }`),
							},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta:    &fnv1beta1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{},
					Conditions: []*fnv1beta1.Condition{
						{
							Type:    "StatusConditionsReady",
							Status:  fnv1beta1.Status_STATUS_CONDITION_FALSE,
							Reason:  "SetConditionFailure",
							Target:  fnv1beta1.Target_TARGET_COMPOSITE.Enum(),
							Message: ptr.To("failed to set condition, statusConditionHookIndex: 0, setConditionIndex: 0: [failed to parse template, template: :1: unexpected \"}\" in operand]"),
						},
					},
				},
			},
		},
		"ContinuesOnFailure": {
			reason: "When encountering an error with a matchCondition, the parent statusConditionHook should be skipped but other statusConditionHooks should still execute. When encountering an error with a setCondition, only that individual setCondition should be skipped.",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`
{
  "apiVersion": "managed-resource-hook.fn.crossplane.io/v1beta1",
  "kind": "ManagedResourceHook",
  "statusConditionHooks": [
    {
      "matchConditions": [
        {
          "resourceName": "example-mr",
          "type": "Synced",
          "status": "False",
          "reason": "ReconcileError",
					"message": "a bad regex (?!)"
        }
      ],
      "setConditions": [
        {
          "type": "CustomReady",
          "status": "False",
          "reason": "InternalError",
          "message": "a matchcondition failed, this should not be set",
          "target": "Composite"
        }
      ]
    },
    {
      "matchConditions": [
        {
          "resourceName": "example-mr",
          "type": "Synced",
          "status": "False",
          "reason": "ReconcileError",
					"message": "Something went wrong: (?P<Error>.+)"
        }
      ],
      "setConditions": [
        {
          "type": "ShouldNotBeSet",
          "status": "False",
          "reason": "InternalError",
          "message": "this condition will fail {{ .Error }",
          "target": "Composite"
        },
        {
          "type": "CustomReady",
          "status": "False",
          "reason": "InternalError",
					"message": "this condition should be set, error: {{ .Error }}",
          "target": "Composite"
        }
      ]
    }
  ]
}
`),
					Observed: &fnv1beta1.State{
						Resources: map[string]*fnv1beta1.Resource{
							"example-mr": {
								Resource: resource.MustStructJSON(`
{
    "apiVersion": "some.example.com/v1alpha1",
    "kind": "Object",
    "metadata": {
      "name": "example-name"
    },
    "status": {
      "conditions": [
        {
					"message": "Something went wrong: some lower level error",
          "reason": "ReconcileError",
          "status": "False",
          "type": "Synced"
        }
      ]
    }
  }`),
							},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta:    &fnv1beta1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{},
					Conditions: []*fnv1beta1.Condition{
						{
							Type:    "StatusConditionsReady",
							Status:  fnv1beta1.Status_STATUS_CONDITION_FALSE,
							Reason:  "MatchFailure",
							Target:  fnv1beta1.Target_TARGET_COMPOSITE.Enum(),
							Message: ptr.To("error when matching, statusConditionHookIndex: 0, matchConditionIndex: 0: [failed to compile message regex, error parsing regexp: invalid or unsupported Perl syntax: `(?!`]"),
						},
						{
							Type:    "StatusConditionsReady",
							Status:  fnv1beta1.Status_STATUS_CONDITION_FALSE,
							Reason:  "SetConditionFailure",
							Target:  fnv1beta1.Target_TARGET_COMPOSITE.Enum(),
							Message: ptr.To("failed to set condition, statusConditionHookIndex: 1, setConditionIndex: 0: [failed to parse template, template: :1: unexpected \"}\" in operand]"),
						},
						{
							Type:    "CustomReady",
							Status:  fnv1beta1.Status_STATUS_CONDITION_FALSE,
							Reason:  "InternalError",
							Target:  fnv1beta1.Target_TARGET_COMPOSITE.Enum(),
							Message: ptr.To("this condition should be set, error: some lower level error"),
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := &Function{log: logging.NewNopLogger()}
			rsp, err := f.RunFunction(tc.args.ctx, tc.args.req)

			if diff := cmp.Diff(tc.want.rsp, rsp, protocmp.Transform()); diff != "" {
				t.Errorf("%s\nf.RunFunction(...): -want rsp, +got rsp:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nf.RunFunction(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}
