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
  "apiVersion": "function-status-transformer.fn.crossplane.io/v1beta1",
  "kind": "ManagedResourceHook",
  "statusConditionHooks": [
    {
      "matchConditions": [
        {
          "resourceName": "example-mr",
					"condition": {
						"type": "Synced",
						"status": "False",
						"reason": "ReconcileError",
						"message": "Something went wrong: (?P<Error>.+)"
					}
        }
      ],
      "setConditions": [
        {
          "target": "Composite",
					"condition": {
						"type": "CustomReady",
						"status": "False",
						"reason": "InternalError",
						"message": "{{ .Error }}"
					}
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
							Type:   "StatusTransformationSuccess",
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
		  "apiVersion": "function-status-transformer.fn.crossplane.io/v1beta1",
		  "kind": "ManagedResourceHook",
		  "statusConditionHooks": [
		    {
		      "matchConditions": [
		        {
		          "resourceName": "example-mr",
							"condition": {
								"type": "Synced",
								"reason": "ReconcileError",
								"message": "Something went wrong."
							}
		        }
		      ],
		      "setConditions": [
		        {
		          "target": "Composite",
							"condition": {
								"type": "NilStatus",
								"status": "True",
								"reason": "Test",
								"message": "Testing wildcard matching."
							}
		        }
		      ]
		    },
		    {
		      "matchConditions": [
		        {
		          "resourceName": "example-mr",
							"condition": {
								"type": "Synced",
								"status": "False",
								"message": "Something went wrong."
							}
		        }
		      ],
		      "setConditions": [
		        {
		          "target": "Composite",
							"condition": {
								"type": "NilReason",
								"status": "True",
								"reason": "Test",
								"message": "Testing wildcard matching."
							}
		        }
		      ]
		    },
		    {
		      "matchConditions": [
		        {
		          "resourceName": "example-mr",
							"condition": {
								"type": "Synced",
								"status": "False",
								"reason": "ReconcileError"
							}
		        }
		      ],
		      "setConditions": [
		        {
		          "target": "Composite",
							"condition": {
								"type": "NilMessage",
								"status": "True",
								"reason": "Test",
								"message": "Testing wildcard matching."
							}
		        }
		      ]
		    },
		    {
		      "matchConditions": [
		        {
		          "resourceName": "example-mr",
							"condition": {
								"type": "Synced"
							}
		        }
		      ],
		      "setConditions": [
		        {
		          "target": "Composite",
							"condition": {
								"type": "NilAll",
								"status": "True",
								"reason": "Test",
								"message": "Testing wildcard matching."
							}
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
							Type:   "StatusTransformationSuccess",
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
		  "apiVersion": "function-status-transformer.fn.crossplane.io/v1beta1",
		  "kind": "ManagedResourceHook",
		  "statusConditionHooks": [
		    {
		      "matchConditions": [
		        {
		          "resourceName": "example-mr",
							"condition": {
								"type": "Synced",
								"status": "False",
								"reason": "ReconcileError"
							}
		        },
		        {
		          "resourceName": "example-mr",
							"condition": {
								"type": "Ready",
								"status": "False",
								"reason": "ReconcileError"
							}
		        }
		      ],
		      "setConditions": [
		        {
		          "target": "Composite",
							"condition": {
								"type": "MatchedBoth",
								"status": "True",
								"reason": "ShouldMatchBoth",
								"message": "Match conditions are ANDed together. Both conditions should match."
							}
		        }
		      ]
		    },
				{
		      "matchConditions": [
		        {
		          "resourceName": "example-mr",
							"condition": {
								"type": "Synced",
								"status": "False",
								"reason": "ReconcileError"
							}
		        },
		        {
		          "resourceName": "example-mr",
							"condition": {
								"type": "DoesNotExist",
								"status": "False",
								"reason": "ReconcileError"
							}
		        }
		      ],
		      "setConditions": [
		        {
		          "target": "Composite",
							"condition": {
								"type": "MatchedOne",
								"status": "True",
								"reason": "OneDidNotMatch",
								"message": "Should not match on the DoesNotExist condition."
							}
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
							Type:   "StatusTransformationSuccess",
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
		  "apiVersion": "function-status-transformer.fn.crossplane.io/v1beta1",
		  "kind": "ManagedResourceHook",
		  "statusConditionHooks": [
		    {
		      "matchConditions": [
		        {
		          "resourceName": "example-mr",
							"condition": {
								"type": "Synced",
								"status": "False",
								"reason": "ReconcileError"
							}
		        }
		      ],
		      "setConditions": [
		        {
		          "target": "CompositeAndClaim",
							"condition": {
								"type": "ConditionA",
								"status": "True",
								"reason": "SetFirst"
							}
		        },
		        {
		          "target": "CompositeAndClaim",
							"condition": {
								"type": "ConditionA",
								"status": "True",
								"reason": "SetSecond",
								"message": "Should not be set as force is not used."
							}
		        },
						{
		          "target": "CompositeAndClaim",
							"condition": {
								"type": "ConditionB",
								"status": "True",
								"reason": "SetFirst"
							}
		        },
						{
		          "target": "CompositeAndClaim",
							"force": true,
							"condition": {
								"type": "ConditionB",
								"status": "True",
								"reason": "SetSecond"
							}
		        }
		      ]
		    },
		    {
		      "matchConditions": [
		        {
		          "resourceName": "example-mr",
							"condition": {
								"type": "Synced",
								"status": "False",
								"reason": "ReconcileError"
							}
		        }
		      ],
		      "setConditions": [
		        {
		          "target": "CompositeAndClaim",
							"condition": {
								"type": "ConditionA",
								"status": "True",
								"reason": "SetSecondSeparateMatch",
								"message": "Should not be set as force is not used."
							}
		        },
						{
		          "target": "CompositeAndClaim",
							"force": true,
							"condition": {
								"type": "ConditionB",
								"status": "True",
								"reason": "SetThird"
							}
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
							Type:   "StatusTransformationSuccess",
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
		  "apiVersion": "function-status-transformer.fn.crossplane.io/v1beta1",
		  "kind": "ManagedResourceHook",
		  "statusConditionHooks": [
		    {
		      "matchConditions": [
		        {
		          "resourceName": "example-mr",
							"condition": {
								"type": "Synced",
								"status": "Unknown",
								"reason": "",
								"message": ""
							}
		        }
		      ],
		      "setConditions": [
		        {
		          "target": "CompositeAndClaim",
							"condition": {
								"type": "CustomReady",
								"status": "False",
								"reason": "DoesNotExist"
							}
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
							Type:   "StatusTransformationSuccess",
							Status: fnv1beta1.Status_STATUS_CONDITION_TRUE,
							Reason: "Available",
							Target: fnv1beta1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
				},
			},
		},
		"NoMatchConditions": {
			reason: "If no match conditions are given, it should match everything.",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`
		{
		  "apiVersion": "function-status-transformer.fn.crossplane.io/v1beta1",
		  "kind": "ManagedResourceHook",
		  "statusConditionHooks": [
		    {
		      "matchConditions": [],
		      "setConditions": [
		        {
		          "target": "CompositeAndClaim",
							"condition": {
								"type": "CustomReady",
								"status": "False",
								"reason": "DoesNotExist"
							}
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
							Type:   "StatusTransformationSuccess",
							Status: fnv1beta1.Status_STATUS_CONDITION_TRUE,
							Reason: "Available",
							Target: fnv1beta1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
				},
			},
		},
		"MatchRegexFailureMessage": {
			reason: "The function should set the shared status condition to false when encountering a regex failure when matching the message.",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`
		{
		  "apiVersion": "function-status-transformer.fn.crossplane.io/v1beta1",
		  "kind": "ManagedResourceHook",
		  "statusConditionHooks": [
		    {
		      "matchConditions": [
		        {
		          "resourceName": "example-mr",
							"condition": {
								"type": "Synced",
								"status": "False",
								"reason": "ReconcileError",
								"message": "a bad regex (?!)"
							}
		        }
		      ],
		      "setConditions": [
		        {
		          "target": "Composite",
							"condition": {
								"type": "CustomReady",
								"status": "False",
								"reason": "InternalError",
								"message": "{{ .Error }}"
							}
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
							Type:    "StatusTransformationSuccess",
							Status:  fnv1beta1.Status_STATUS_CONDITION_FALSE,
							Reason:  "MatchFailure",
							Target:  fnv1beta1.Target_TARGET_COMPOSITE.Enum(),
							Message: ptr.To("error when matching, statusConditionHookIndex: 0, matchConditionIndex: 0: [failed to compile message regex, error parsing regexp: invalid or unsupported Perl syntax: `(?!`]"),
						},
					},
				},
			},
		},
		"MatchRegexFailureResourceName": {
			reason: "The function should set the shared status condition to false when encountering a regex failure when matching the resourceName.",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`
		{
		  "apiVersion": "function-status-transformer.fn.crossplane.io/v1beta1",
		  "kind": "ManagedResourceHook",
		  "statusConditionHooks": [
		    {
		      "matchConditions": [
		        {
		          "resourceName": "example-(?!)",
							"condition": {
								"type": "Synced",
								"status": "False",
								"reason": "ReconcileError"
							}
		        }
		      ],
		      "setConditions": [
		        {
		          "target": "Composite",
							"condition": {
								"type": "CustomReady",
								"status": "False",
								"reason": "InternalError",
								"message": "{{ .Error }}"
							}
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
							Type:    "StatusTransformationSuccess",
							Status:  fnv1beta1.Status_STATUS_CONDITION_FALSE,
							Reason:  "MatchFailure",
							Target:  fnv1beta1.Target_TARGET_COMPOSITE.Enum(),
							Message: ptr.To("error when matching, statusConditionHookIndex: 0, matchConditionIndex: 0: [failed to compile resourceName regex, error parsing regexp: invalid or unsupported Perl syntax: `(?!`]"),
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
		  "apiVersion": "function-status-transformer.fn.crossplane.io/v1beta1",
		  "kind": "ManagedResourceHook",
		  "statusConditionHooks": [
		    {
		      "matchConditions": [
		        {
		          "resourceName": "example-mr",
							"condition": {
								"type": "Synced",
								"status": "False",
								"reason": "ReconcileError",
								"message": "Something went wrong: (?P<Error>.+)"
							}
		        }
		      ],
		      "setConditions": [
		        {
		          "target": "Composite",
							"condition": {
								"type": "CustomReady",
								"status": "False",
								"reason": "InternalError",
								"message": "{{ .Error }"
							}
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
							Type:    "StatusTransformationSuccess",
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
		  "apiVersion": "function-status-transformer.fn.crossplane.io/v1beta1",
		  "kind": "ManagedResourceHook",
		  "statusConditionHooks": [
		    {
		      "matchConditions": [
		        {
		          "resourceName": "example-mr",
							"condition": {
								"type": "Synced",
								"status": "False",
								"reason": "ReconcileError",
								"message": "a bad regex (?!)"
							}
		        }
		      ],
		      "setConditions": [
		        {
		          "target": "Composite",
							"condition": {
								"type": "CustomReady",
								"status": "False",
								"reason": "InternalError",
								"message": "a matchcondition failed, this should not be set"
							}
		        }
		      ]
		    },
		    {
		      "matchConditions": [
		        {
		          "resourceName": "example-mr",
							"condition": {
								"type": "Synced",
								"status": "False",
								"reason": "ReconcileError",
								"message": "Something went wrong: (?P<Error>.+)"
							}
		        }
		      ],
		      "setConditions": [
		        {
		          "target": "Composite",
							"condition": {
								"type": "ShouldNotBeSet",
								"status": "False",
								"reason": "InternalError",
								"message": "this condition will fail {{ .Error }"
							}
		        },
		        {
		          "target": "Composite",
							"condition": {
								"type": "CustomReady",
								"status": "False",
								"reason": "InternalError",
								"message": "this condition should be set, error: {{ .Error }}"
							}
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
							Type:    "StatusTransformationSuccess",
							Status:  fnv1beta1.Status_STATUS_CONDITION_FALSE,
							Reason:  "MatchFailure",
							Target:  fnv1beta1.Target_TARGET_COMPOSITE.Enum(),
							Message: ptr.To("error when matching, statusConditionHookIndex: 0, matchConditionIndex: 0: [failed to compile message regex, error parsing regexp: invalid or unsupported Perl syntax: `(?!`]"),
						},
						{
							Type:    "StatusTransformationSuccess",
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
		"ResourceNameRegex": {
			reason: "The function should treat resource names as regular expressions so that a matchCondition can target multiple resources.",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`
		{
		  "apiVersion": "function-status-transformer.fn.crossplane.io/v1beta1",
		  "kind": "ManagedResourceHook",
		  "statusConditionHooks": [
		    {
		      "matchConditions": [
		        {
		          "resourceName": "database-\\w+",
							"condition": {
								"type": "Synced",
								"status": "False",
								"reason": "ReconcileError",
								"message": "Something went wrong: (?P<Error>.+)"
							}
		        }
		      ],
		      "setConditions": [
		        {
		          "target": "Composite",
							"condition": {
								"type": "DefaultMatchType",
								"status": "True",
								"reason": "TestingDefaultMatchAll",
								"message": "Should not exist because database-a will not match."
							}
		        }
		      ]
		    },
		    {
		      "matchConditions": [
		        {
		          "resourceName": "database-\\w+",
							"type": "MatchAll",
							"condition": {
								"type": "Synced",
								"status": "False",
								"reason": "ReconcileError",
								"message": "Something went wrong: (?P<Error>.+)"
							}
		        }
		      ],
		      "setConditions": [
		        {
		          "target": "Composite",
							"condition": {
								"type": "MatchedAll",
								"status": "True",
								"reason": "TestingMatchAll",
								"message": "Should not exist because database-a will not match."
							}
		        }
		      ]
		    },
		    {
		      "matchConditions": [
		        {
		          "resourceName": "database-\\w+",
							"type": "MatchAny",
							"condition": {
								"type": "Synced",
								"status": "False",
								"reason": "ReconcileError",
								"message": "Something went wrong: (?P<Error>.+)"
							}
		        }
		      ],
		      "setConditions": [
		        {
		          "target": "Composite",
							"condition": {
								"type": "MatchedAny",
								"status": "True",
								"reason": "TestingMatchAny",
								"message": "Matched error: {{ .Error }}"
							}
		        }
		      ]
		    }
		  ]
		}
		`),
					Observed: &fnv1beta1.State{
						Resources: map[string]*fnv1beta1.Resource{
							"database-a": {
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
		          "reason": "Available",
		          "status": "True",
		          "type": "Synced"
		        }
		      ]
		    }
		}`),
							},
							"database-b": {
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
							Type:    "MatchedAny",
							Status:  fnv1beta1.Status_STATUS_CONDITION_TRUE,
							Reason:  "TestingMatchAny",
							Message: ptr.To("Matched error: some lower level error"),
							Target:  fnv1beta1.Target_TARGET_COMPOSITE.Enum(),
						},
						{
							Type:   "StatusTransformationSuccess",
							Status: fnv1beta1.Status_STATUS_CONDITION_TRUE,
							Reason: "Available",
							Target: fnv1beta1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
				},
			},
		},
		"BadInput": {
			reason: "The function should fail if the input cannot be parsed.",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`
		{
						"object": "not valid"
		}
		`),
					Observed: &fnv1beta1.State{
						Resources: map[string]*fnv1beta1.Resource{},
					},
				},
			},
			want: want{
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta: &fnv1beta1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1beta1.Condition{
						{
							Type:    "StatusTransformationSuccess",
							Status:  fnv1beta1.Status_STATUS_CONDITION_FALSE,
							Reason:  "InputFailure",
							Message: ptr.To("cannot get Function input from *v1beta1.RunFunctionRequest: cannot get function input *v1beta1.StatusTransformation from *v1beta1.RunFunctionRequest: cannot unmarshal JSON from *structpb.Struct into *v1beta1.StatusTransformation: json: cannot unmarshal Go value of type v1beta1.StatusTransformation: unknown name \"object\""),
							Target:  fnv1beta1.Target_TARGET_COMPOSITE.Enum(),
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
