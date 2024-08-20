package main

import (
	"context"
	"strings"
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
		rsp        *fnv1beta1.RunFunctionResponse
		cleanError bool
		err        error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"CapturesRegexGroups": {
			reason: "The function should be able to capture regex groups and use them when setting conditions.",
			args: args{
				ctx: context.Background(),
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`
{
  "apiVersion": "function-status-transformer.fn.crossplane.io/v1beta1",
  "kind": "StatusTransformation",
  "statusConditionHooks": [
    {
      "matchers": [
        {
          "resources": [
            {
              "name": "example-mr"
            }
          ],
          "conditions": [
            {
              "type": "Synced",
              "status": "False",
              "reason": "ReconcileError",
              "message": "Something went wrong: (?P<Error>.+)"
            }
          ]
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
      ],
      "createEvents": [
        {
          "target": "Composite",
          "event": {
            "type": "Normal",
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
					Meta: &fnv1beta1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{
						{
							Severity: fnv1beta1.Severity_SEVERITY_NORMAL,
							Message:  "some lower level error",
							Reason:   ptr.To("InternalError"),
							Target:   fnv1beta1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
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
				ctx: context.TODO(),
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`
{
  "apiVersion": "function-status-transformer.fn.crossplane.io/v1beta1",
  "kind": "StatusTransformation",
  "statusConditionHooks": [
    {
      "matchers": [
        {
          "resources": [
            {
              "name": "example-mr"
            }
          ],
          "conditions": [
            {
              "type": "Synced",
              "reason": "ReconcileError",
              "message": "Something went wrong."
            }
          ]
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
      "matchers": [
        {
          "resources": [
            {
              "name": "example-mr"
            }
          ],
          "conditions": [
            {
              "type": "Synced",
              "status": "False",
              "message": "Something went wrong."
            }
          ]
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
      "matchers": [
        {
          "resources": [
            {
              "name": "example-mr"
            }
          ],
          "conditions": [
            {
              "type": "Synced",
              "status": "False",
              "reason": "ReconcileError"
            }
          ]
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
      "matchers": [
        {
          "resources": [
            {
              "name": "example-mr"
            }
          ],
          "conditions": [
            {
              "type": "Synced"
            }
          ]
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
				ctx: context.TODO(),
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`
{
  "apiVersion": "function-status-transformer.fn.crossplane.io/v1beta1",
  "kind": "StatusTransformation",
  "statusConditionHooks": [
    {
      "matchers": [
        {
          "resources": [
            {
              "name": "example-mr"
            }
          ],
          "conditions": [
            {
              "type": "Synced",
              "status": "False",
              "reason": "ReconcileError"
            }
          ]
        },
        {
          "resources": [
            {
              "name": "example-mr"
            }
          ],
          "conditions": [
            {
              "type": "Ready",
              "status": "False",
              "reason": "ReconcileError"
            }
          ]
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
      "matchers": [
        {
          "resources": [
            {
              "name": "example-mr"
            }
          ],
          "conditions": [
            {
              "type": "Synced",
              "status": "False",
              "reason": "ReconcileError"
            }
          ]
        },
        {
          "resources": [
            {
              "name": "example-mr"
            }
          ],
          "conditions": [
            {
              "type": "DoesNotExist",
              "status": "False",
              "reason": "ReconcileError"
            }
          ]
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
				ctx: context.TODO(),
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`
{
  "apiVersion": "function-status-transformer.fn.crossplane.io/v1beta1",
  "kind": "StatusTransformation",
  "statusConditionHooks": [
    {
      "matchers": [
        {
          "resources": [
            {
              "name": "example-mr"
            }
          ],
          "conditions": [
            {
              "type": "Synced",
              "status": "False",
              "reason": "ReconcileError"
            }
          ]
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
      "matchers": [
        {
          "resources": [
            {
              "name": "example-mr"
            }
          ],
          "conditions": [
            {
              "type": "Synced",
              "status": "False",
              "reason": "ReconcileError"
            }
          ]
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
				ctx: context.TODO(),
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`
{
  "apiVersion": "function-status-transformer.fn.crossplane.io/v1beta1",
  "kind": "StatusTransformation",
  "statusConditionHooks": [
    {
      "matchers": [
        {
          "resources": [
            {
              "name": "example-mr"
            }
          ],
          "conditions": [
            {
              "type": "Synced",
              "status": "Unknown",
              "reason": "",
              "message": ""
            }
          ]
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
		      "conditions": []
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
			reason: "If no match conditions are given, it should not match anything.",
			args: args{
				ctx: context.TODO(),
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`
				{
				  "apiVersion": "function-status-transformer.fn.crossplane.io/v1beta1",
				  "kind": "StatusTransformation",
				  "statusConditionHooks": [
				    {
				      "matchers": [],
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
							Type:   "StatusTransformationSuccess",
							Status: fnv1beta1.Status_STATUS_CONDITION_TRUE,
							Reason: "Available",
							Target: fnv1beta1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
				},
			},
		},
		"MatchConditionNoResource": {
			reason: "If a match condition does not find a resource to match against, it should evaluate to false.",
			args: args{
				ctx: context.TODO(),
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`
{
  "apiVersion": "function-status-transformer.fn.crossplane.io/v1beta1",
  "kind": "StatusTransformation",
  "statusConditionHooks": [
    {
      "matchers": [
        {
          "resources": [
            {
              "name": "resource-key-a"
            }
          ],
          "conditions": [
            {
              "message": "Something went wrong: some lower level error",
              "reason": "ReconcileError",
              "status": "False",
              "type": "Synced"
            }
          ]
        }
      ],
      "setConditions": [
        {
          "target": "CompositeAndClaim",
          "condition": {
            "type": "CustomReady",
            "status": "False",
            "reason": "ResourceNotFound"
          }
        }
      ]
    }
  ]
}
				`),
					Observed: &fnv1beta1.State{
						Resources: map[string]*fnv1beta1.Resource{
							"resource-key-b": {
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
							Type:   "StatusTransformationSuccess",
							Status: fnv1beta1.Status_STATUS_CONDITION_TRUE,
							Reason: "Available",
							Target: fnv1beta1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
				},
			},
		},
		"SettingDefaultConditions": {
			reason: "Users should be able to effectively set a default condition by providing a matchCondition of a custom type that matches the default values. They will still be required to match on the resourceKey.",
			args: args{
				ctx: context.TODO(),
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`
{
  "apiVersion": "function-status-transformer.fn.crossplane.io/v1beta1",
  "kind": "StatusTransformation",
  "statusConditionHooks": [
    {
      "matchers": [
        {
          "resources": [
            {
              "name": "resource-key-a"
            }
          ],
          "conditions": [
            {
              "message": "",
              "reason": "",
              "status": "Unknown",
              "type": "ThisTypeDoesNotExist"
            }
          ]
        }
      ],
      "setConditions": [
        {
          "target": "CompositeAndClaim",
          "condition": {
            "type": "CustomReady",
            "status": "False",
            "reason": "DefaultCondition"
          }
        }
      ]
    },
    {
      "matchers": [
        {
          "resources": [
            {
              "name": "resource-key-does-not-match"
            }
          ],
          "conditions": [
            {
              "message": "",
              "reason": "",
              "status": "Unknown",
              "type": "ThisTypeDoesNotExist"
            }
          ]
        }
      ],
      "setConditions": [
        {
          "target": "CompositeAndClaim",
          "condition": {
            "type": "CustomReadyShouldNotMatch",
            "status": "False",
            "reason": "DefaultCondition",
            "message": "This condition should not be set as the resourceKey does not match a resource."
          }
        }
      ]
    }
  ]
}
				`),
					Observed: &fnv1beta1.State{
						Resources: map[string]*fnv1beta1.Resource{
							"resource-key-a": {
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
							Reason: "DefaultCondition",
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
				ctx: context.TODO(),
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`
{
  "apiVersion": "function-status-transformer.fn.crossplane.io/v1beta1",
  "kind": "StatusTransformation",
  "statusConditionHooks": [
    {
      "matchers": [
        {
          "resources": [
            {
              "name": "example-mr"
            }
          ],
          "conditions": [
            {
              "type": "Synced",
              "status": "False",
              "reason": "ReconcileError",
              "message": "a bad regex (?!)"
            }
          ]
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
							Message: ptr.To("cannot match resources, statusConditionHookIndex: 0, matchConditionIndex: 0: cannot compile message regex: error parsing regexp: invalid or unsupported Perl syntax: `(?!`"),
						},
					},
				},
			},
		},
		"MatchRegexFailureResourceName": {
			reason: "The function should set the shared status condition to false when encountering a regex failure when matching the resourceName.",
			args: args{
				ctx: context.TODO(),
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`
{
  "apiVersion": "function-status-transformer.fn.crossplane.io/v1beta1",
  "kind": "StatusTransformation",
  "statusConditionHooks": [
    {
      "matchers": [
        {
          "resources": [
            {
              "name": "example-(?!)"
            }
          ],
          "conditions": [
            {
              "type": "Synced",
              "status": "False",
              "reason": "ReconcileError"
            }
          ]
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
							Message: ptr.To("cannot match resources, statusConditionHookIndex: 0, matchConditionIndex: 0: cannot compile resource key regex, resourcesIndex: 0: error parsing regexp: invalid or unsupported Perl syntax: `(?!`"),
						},
					},
				},
			},
		},
		"TemplateParseFailure": {
			reason: "The function should set the shared status condition to false when encountering a template parsing error.",
			args: args{
				ctx: context.TODO(),
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`
{
  "apiVersion": "function-status-transformer.fn.crossplane.io/v1beta1",
  "kind": "StatusTransformation",
  "statusConditionHooks": [
    {
      "matchers": [
        {
          "resources": [
            {
              "name": "example-mr"
            }
          ],
          "conditions": [
            {
              "type": "Synced",
              "status": "False",
              "reason": "ReconcileError",
              "message": "Something went wrong: (?P<Error>.+)"
            }
          ]
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
							Message: ptr.To("cannot set condition, statusConditionHookIndex: 0, setConditionIndex: 0: cannot parse template: template: :1: unexpected \"}\" in operand"),
						},
					},
				},
			},
		},
		"ContinuesOnFailure": {
			reason: "When encountering an error with a matchCondition, the parent statusConditionHook should be skipped but other statusConditionHooks should still execute. When encountering an error with a setCondition, only that individual setCondition should be skipped.",
			args: args{
				ctx: context.TODO(),
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`
{
  "apiVersion": "function-status-transformer.fn.crossplane.io/v1beta1",
  "kind": "StatusTransformation",
  "statusConditionHooks": [
    {
      "matchers": [
        {
          "resources": [
            {
              "name": "example-mr"
            }
          ],
          "conditions": [
            {
              "type": "Synced",
              "status": "False",
              "reason": "ReconcileError",
              "message": "a bad regex (?!)"
            }
          ]
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
      "matchers": [
        {
          "resources": [
            {
              "name": "example-mr"
            }
          ],
          "conditions": [
            {
              "type": "Synced",
              "status": "False",
              "reason": "ReconcileError",
              "message": "Something went wrong: (?P<Error>.+)"
            }
          ]
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
							Message: ptr.To("cannot match resources, statusConditionHookIndex: 0, matchConditionIndex: 0: cannot compile message regex: error parsing regexp: invalid or unsupported Perl syntax: `(?!`"),
						},
						{
							Type:    "StatusTransformationSuccess",
							Status:  fnv1beta1.Status_STATUS_CONDITION_FALSE,
							Reason:  "SetConditionFailure",
							Target:  fnv1beta1.Target_TARGET_COMPOSITE.Enum(),
							Message: ptr.To("cannot set condition, statusConditionHookIndex: 1, setConditionIndex: 0: cannot parse template: template: :1: unexpected \"}\" in operand"),
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
		"BadInput": {
			reason: "The function should fail if the input cannot be parsed.",
			args: args{
				ctx: context.TODO(),
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
				cleanError: true,
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
		"InvalidEventType": {
			reason: "The function should set a non-successful status if it encounters an event with an invalid type.",
			args: args{
				ctx: context.TODO(),
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`
{
  "apiVersion": "function-status-transformer.fn.crossplane.io/v1beta1",
  "kind": "StatusTransformation",
  "statusConditionHooks": [
    {
      "matchers": [
        {
          "resources": [
            {
              "name": "example-mr"
            }
          ],
          "conditions": [
            {
              "type": "Synced",
              "status": "False",
              "reason": "ReconcileError",
              "message": "Something went wrong: (?P<Error>.+)"
            }
          ]
        }
      ],
      "createEvents": [
        {
          "target": "Composite",
          "event": {
            "type": "ThisIsAnInvalidType",
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
							Reason:  "SetConditionFailure",
							Target:  fnv1beta1.Target_TARGET_COMPOSITE.Enum(),
							Message: ptr.To("cannot create event, statusConditionHookIndex: 0, createEventIndex: 0: invalid type ThisIsAnInvalidType, must be one of [Normal, Warning]"),
						},
					},
				},
			},
		},
		"DefaultEventType": {
			reason: "If no event type is given, it should default to normal.",
			args: args{
				ctx: context.TODO(),
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`
{
  "apiVersion": "function-status-transformer.fn.crossplane.io/v1beta1",
  "kind": "StatusTransformation",
  "statusConditionHooks": [
    {
      "matchers": [
        {
          "resources": [
            {
              "name": "example-mr"
            }
          ],
          "conditions": [
            {
              "type": "Synced",
              "status": "False",
              "reason": "ReconcileError",
              "message": "Something went wrong: (?P<Error>.+)"
            }
          ]
        }
      ],
      "createEvents": [
        {
          "target": "Composite",
          "event": {
            "reason": "InternalError",
            "message": "Some message."
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
					Meta: &fnv1beta1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{
						{
							Severity: fnv1beta1.Severity_SEVERITY_NORMAL,
							Message:  "Some message.",
							Reason:   ptr.To("InternalError"),
							Target:   fnv1beta1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Conditions: []*fnv1beta1.Condition{
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
		"DefaultTargets": {
			reason: "Target should be an optional field. Should default to Composite.",
			args: args{
				ctx: context.TODO(),
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`
{
  "apiVersion": "function-status-transformer.fn.crossplane.io/v1beta1",
  "kind": "StatusTransformation",
  "statusConditionHooks": [
    {
      "matchers": [
        {
          "resources": [
            {
              "name": "example-mr"
            }
          ],
          "conditions": [
            {
              "type": "Synced",
              "status": "False",
              "reason": "ReconcileError",
              "message": "Something went wrong: (?P<Error>.+)"
            }
          ]
        }
      ],
      "setConditions": [
        {
          "condition": {
            "type": "CustomReady",
            "status": "False",
            "reason": "InternalError",
            "message": "{{ .Error }}"
          }
        }
      ],
      "createEvents": [
        {
          "event": {
            "reason": "InternalError",
            "message": "Some message."
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
					Meta: &fnv1beta1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{
						{
							Severity: fnv1beta1.Severity_SEVERITY_NORMAL,
							Message:  "Some message.",
							Reason:   ptr.To("InternalError"),
							Target:   fnv1beta1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Conditions: []*fnv1beta1.Condition{
						{
							Type:    "CustomReady",
							Status:  fnv1beta1.Status_STATUS_CONDITION_FALSE,
							Reason:  "InternalError",
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
		"AnyResourceMatchesAnyCondition": {
			reason: "AnyResourceMatchesAnyCondition should behave as expected.",
			args: args{
				ctx: context.TODO(),
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`
{
  "apiVersion": "function-status-transformer.fn.crossplane.io/v1beta1",
  "kind": "StatusTransformation",
  "statusConditionHooks": [
    {
      "matchers": [
        {
          "type": "AnyResourceMatchesAnyCondition",
          "resources": [
            {
              "name": "resource-a"
            },
            {
              "name": "resource-b"
            }
          ],
          "conditions": [
            {
              "type": "Synced",
              "status": "True",
              "reason": "ReconcileError"
            },
            {
              "type": "Synced",
              "status": "False",
              "reason": "ReconcileError"
            }
          ]
        }
      ],
      "setConditions": [
        {
          "condition": {
            "type": "ShouldBeSet",
            "status": "True",
            "reason": "Testing",
            "message": "This condition should be set because the second condition matches resource-b."
          }
        }
      ]
    },
    {
      "matchers": [
        {
          "type": "AnyResourceMatchesAnyCondition",
          "resources": [
            {
              "name": "resource-a"
            },
            {
              "name": "resource-b"
            }
          ],
          "conditions": [
            {
              "type": "Synced",
              "status": "True",
              "reason": "DoesNotMatch"
            },
            {
              "type": "Synced",
              "status": "False",
              "reason": "DoesNotMatch"
            }
          ]
        }
      ],
      "setConditions": [
        {
          "condition": {
            "type": "CustomReady",
            "status": "False",
            "reason": "InternalError",
            "message": "This condition should to be set because there should be no match."
          }
        }
      ]
    },
    {
      "matchers": [
        {
          "type": "AnyResourceMatchesAnyCondition",
          "resources": [],
          "conditions": [
            {
              "type": "Synced",
              "status": "False"
            }
          ]
        }
      ],
      "setConditions": [
        {
          "condition": {
            "type": "ShouldNotBeSet",
            "status": "True",
            "reason": "Testing",
            "message": "This condition should not be set because it does not match against any resources."
          }
        }
      ]
    },
    {
      "matchers": [
        {
          "type": "AnyResourceMatchesAnyCondition",
          "resources": [
            {
              "name": "resource-a"
            },
            {
              "name": "resource-b"
            }
          ],
          "conditions": []
        }
      ],
      "setConditions": [
        {
          "condition": {
            "type": "ShouldNotBeSet",
            "status": "True",
            "reason": "Testing",
            "message": "This condition should not be set because it does not match against any conditions."
          }
        }
      ]
    }
  ]
}
						`),
					Observed: &fnv1beta1.State{
						Resources: map[string]*fnv1beta1.Resource{
							// Resource A will not match to ensure it is treated as ANY and
							// not ALL.
							"resource-a": {
								Resource: resource.MustStructJSON(`
		{
		    "apiVersion": "some.example.com/v1alpha1",
		    "kind": "Object",
		    "metadata": {
		      "name": "example-name"
		    },
		    "status": {
		      "conditions": []
		    }
		  }`),
							},
							"resource-b": {
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
					Meta: &fnv1beta1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1beta1.Condition{
						{
							Type:    "ShouldBeSet",
							Status:  fnv1beta1.Status_STATUS_CONDITION_TRUE,
							Reason:  "Testing",
							Message: ptr.To("This condition should be set because the second condition matches resource-b."),
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
		"AnyResourceMatchesAllConditions": {
			reason: "AnyResourceMatchesAllConditions should behave as expected.",
			args: args{
				ctx: context.TODO(),
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`
{
  "apiVersion": "function-status-transformer.fn.crossplane.io/v1beta1",
  "kind": "StatusTransformation",
  "statusConditionHooks": [
    {
      "matchers": [
        {
          "type": "AnyResourceMatchesAllConditions",
          "resources": [
            {
              "name": "resource-a"
            },
            {
              "name": "resource-b"
            }
          ],
          "conditions": [
            {
              "type": "Synced",
              "status": "True"
            },
            {
              "type": "Ready",
              "status": "True"
            }
          ]
        }
      ],
      "setConditions": [
        {
          "condition": {
            "type": "ShouldBeSet",
            "status": "True",
            "reason": "Testing",
            "message": "This condition should be set. All conditions are matched by resource-a."
          }
        }
      ]
    },
    {
      "matchers": [
        {
          "type": "AnyResourceMatchesAllConditions",
          "resources": [
            {
              "name": "resource-a"
            },
            {
              "name": "resource-b"
            }
          ],
          "conditions": [
            {
              "type": "Synced",
              "status": "True"
            },
            {
              "type": "Ready",
              "status": "False",
              "message": "This condition does not match."
            }
          ]
        }
      ],
      "setConditions": [
        {
          "condition": {
            "type": "ShouldNotBeSet",
            "status": "True",
            "reason": "Testing",
            "message": "This condition should not be set because only one condition matches."
          }
        }
      ]
    },
		{
      "matchers": [
        {
          "type": "AnyResourceMatchesAllConditions",
          "resources": [],
          "conditions": [
            {
              "type": "Synced",
              "status": "False"
            }
          ]
        }
      ],
      "setConditions": [
        {
          "condition": {
            "type": "ShouldNotBeSet",
            "status": "True",
            "reason": "Testing",
            "message": "This condition should not be set because it does not match against any resources."
          }
        }
      ]
    },
    {
      "matchers": [
        {
          "type": "AnyResourceMatchesAllConditions",
          "resources": [
            {
              "name": "resource-a"
            },
            {
              "name": "resource-b"
            }
          ],
          "conditions": []
        }
      ],
      "setConditions": [
        {
          "condition": {
            "type": "ShouldNotBeSet",
            "status": "True",
            "reason": "Testing",
            "message": "This condition should not be set because it does not match against any conditions."
          }
        }
      ]
    }
  ]
}
		`),
					Observed: &fnv1beta1.State{
						Resources: map[string]*fnv1beta1.Resource{
							"resource-a": {
								Resource: resource.MustStructJSON(`
{
  "apiVersion": "some.example.com/v1alpha1",
  "kind": "Object",
  "metadata": {
    "name": "resource-name-a"
  },
  "status": {
    "conditions": [
      {
        "type": "Synced",
        "status": "True",
        "reason": "ReconcileSuccess"
      },
      {
        "type": "Ready",
        "status": "True",
        "reason": "Available"
      }
    ]
  }
}
`),
							},
							"resource-b": {
								Resource: resource.MustStructJSON(`
{
  "apiVersion": "some.example.com/v1alpha1",
  "kind": "Object",
  "metadata": {
    "name": "resource-name-b"
  },
  "status": {
    "conditions": [
      {
        "type": "Synced",
        "status": "False",
        "reason": "ReconcileError"
      }
    ]
  }
}
`),
							},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta: &fnv1beta1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1beta1.Condition{
						{
							Type:    "ShouldBeSet",
							Status:  fnv1beta1.Status_STATUS_CONDITION_TRUE,
							Reason:  "Testing",
							Message: ptr.To("This condition should be set. All conditions are matched by resource-a."),
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
		"AllResourcesMatchAnyCondition": {
			reason: "AllResourcesMatchAnyCondition should behave as expected.",
			args: args{
				ctx: context.TODO(),
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`
{
  "apiVersion": "function-status-transformer.fn.crossplane.io/v1beta1",
  "kind": "StatusTransformation",
  "statusConditionHooks": [
    {
      "matchers": [
        {
          "type": "AllResourcesMatchAnyCondition",
          "resources": [
            {
              "name": "resource-a"
            },
            {
              "name": "resource-b"
            }
          ],
          "conditions": [
            {
              "type": "Synced",
              "status": "False"
            },
            {
              "type": "Ready",
              "status": "True"
            }
          ]
        }
      ],
      "setConditions": [
        {
          "condition": {
            "type": "ShouldBeSet",
            "status": "True",
            "reason": "Testing",
            "message": "This condition should be set. All resources are matched by the second condition."
          }
        }
      ]
    },
    {
      "matchers": [
        {
          "type": "AllResourcesMatchAnyCondition",
          "resources": [
            {
              "name": "resource-a"
            },
            {
              "name": "resource-b"
            },
            {
              "name": "resource-c"
            }
          ],
          "conditions": [
            {
              "type": "Synced",
              "status": "False"
            },
            {
              "type": "Ready",
              "status": "True"
            }
          ]
        }
      ],
      "setConditions": [
        {
          "condition": {
            "type": "ShouldNotBeSet",
            "status": "True",
            "reason": "Testing",
            "message": "This condition should not be set because resource-c does not match."
          }
        }
      ]
    },
    {
      "matchers": [
        {
          "type": "AllResourcesMatchAnyCondition",
          "resources": [],
          "conditions": [
            {
              "type": "Synced",
              "status": "False"
            }
          ]
        }
      ],
      "setConditions": [
        {
          "condition": {
            "type": "ShouldNotBeSet",
            "status": "True",
            "reason": "Testing",
            "message": "This condition should not be set because it does not match against any resources."
          }
        }
      ]
    },
    {
      "matchers": [
        {
          "type": "AllResourcesMatchAnyCondition",
          "resources": [
            {
              "name": "resource-a"
            },
            {
              "name": "resource-b"
            }
          ],
          "conditions": []
        }
      ],
      "setConditions": [
        {
          "condition": {
            "type": "ShouldNotBeSet",
            "status": "True",
            "reason": "Testing",
            "message": "This condition should not be set because it does not match against any conditions."
          }
        }
      ]
    }
  ]
}
		`),
					Observed: &fnv1beta1.State{
						Resources: map[string]*fnv1beta1.Resource{
							"resource-a": {
								Resource: resource.MustStructJSON(`
{
  "apiVersion": "some.example.com/v1alpha1",
  "kind": "Object",
  "metadata": {
    "name": "resource-name-a"
  },
  "status": {
    "conditions": [
      {
        "type": "Synced",
        "status": "True",
        "reason": "ReconcileSuccess"
      },
      {
        "type": "Ready",
        "status": "True",
        "reason": "Available"
      }
    ]
  }
}
`),
							},
							"resource-b": {
								Resource: resource.MustStructJSON(`
{
  "apiVersion": "some.example.com/v1alpha1",
  "kind": "Object",
  "metadata": {
    "name": "resource-name-b"
  },
  "status": {
    "conditions": [
      {
        "type": "Synced",
        "status": "True",
        "reason": "ReconcileSuccess"
      },
      {
        "type": "Ready",
        "status": "True",
        "reason": "Available"
      }
    ]
  }
}
`),
							},
							"resource-c": {
								Resource: resource.MustStructJSON(`
{
  "apiVersion": "some.example.com/v1alpha1",
  "kind": "Object",
  "metadata": {
    "name": "resource-name-c"
  },
  "status": {
    "conditions": [
      {
        "type": "Synced",
        "status": "True",
        "reason": "ReconcileSuccess"
      },
      {
        "type": "Ready",
        "status": "False",
        "reason": "Creating"
      }
    ]
  }
}
`),
							},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta: &fnv1beta1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1beta1.Condition{
						{
							Type:    "ShouldBeSet",
							Status:  fnv1beta1.Status_STATUS_CONDITION_TRUE,
							Reason:  "Testing",
							Message: ptr.To("This condition should be set. All resources are matched by the second condition."),
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
		"AllResourcesMatchAllConditions": {
			reason: "AllResourcesMatchAllConditions should behave as expected.",
			args: args{
				ctx: context.TODO(),
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`
{
  "apiVersion": "function-status-transformer.fn.crossplane.io/v1beta1",
  "kind": "StatusTransformation",
  "statusConditionHooks": [
    {
      "matchers": [
        {
          "type": "AllResourcesMatchAllConditions",
          "resources": [
            {
              "name": "resource-a"
            },
            {
              "name": "resource-b"
            }
          ],
          "conditions": [
            {
              "type": "Synced",
              "status": "True"
            },
            {
              "type": "Ready",
              "status": "True"
            }
          ]
        }
      ],
      "setConditions": [
        {
          "condition": {
            "type": "ShouldBeSet",
            "status": "True",
            "reason": "Testing",
            "message": "This condition should be set. All resources match all conditions."
          }
        }
      ]
    },
    {
      "matchers": [
        {
          "type": "AllResourcesMatchAllConditions",
          "resources": [
            {
              "name": "resource-a"
            },
            {
              "name": "resource-b"
            }
          ],
          "conditions": [
            {
              "type": "Synced",
              "status": "True"
            },
            {
              "type": "Ready",
              "status": "False"
            }
          ]
        }
      ],
      "setConditions": [
        {
          "condition": {
            "type": "ShouldNotBeSet",
            "status": "True",
            "reason": "Testing",
            "message": "This condition should not be set. The second condition does not match all resources."
          }
        }
      ]
    },
    {
      "matchers": [
        {
          "type": "AllResourcesMatchAllConditions",
          "resources": [
            {
              "name": "resource-a"
            },
            {
              "name": "resource-b"
            },
            {
              "name": "resource-c"
            }
          ],
          "conditions": [
            {
              "type": "Synced",
              "status": "True"
            },
            {
              "type": "Ready",
              "status": "True"
            }
          ]
        }
      ],
      "setConditions": [
        {
          "condition": {
            "type": "ShouldNotBeSet",
            "status": "True",
            "reason": "Testing",
            "message": "This condition should not be set. resource-c does not match the conditions."
          }
        }
      ]
    },
    {
      "matchers": [
        {
          "type": "AllResourcesMatchAllConditions",
          "resources": [],
          "conditions": [
            {
              "type": "Synced",
              "status": "False"
            }
          ]
        }
      ],
      "setConditions": [
        {
          "condition": {
            "type": "ShouldNotBeSet",
            "status": "True",
            "reason": "Testing",
            "message": "This condition should not be set because it does not match against any resources."
          }
        }
      ]
    },
    {
      "matchers": [
        {
          "type": "AllResourcesMatchAllConditions",
          "resources": [
            {
              "name": "resource-a"
            },
            {
              "name": "resource-b"
            }
          ],
          "conditions": []
        }
      ],
      "setConditions": [
        {
          "condition": {
            "type": "ShouldNotBeSet",
            "status": "True",
            "reason": "Testing",
            "message": "This condition should not be set because it does not match against any conditions."
          }
        }
      ]
    }
  ]
}
		`),
					Observed: &fnv1beta1.State{
						Resources: map[string]*fnv1beta1.Resource{
							"resource-a": {
								Resource: resource.MustStructJSON(`
{
  "apiVersion": "some.example.com/v1alpha1",
  "kind": "Object",
  "metadata": {
    "name": "resource-name-a"
  },
  "status": {
    "conditions": [
      {
        "type": "Synced",
        "status": "True",
        "reason": "ReconcileSuccess"
      },
      {
        "type": "Ready",
        "status": "True",
        "reason": "Available"
      }
    ]
  }
}
`),
							},
							"resource-b": {
								Resource: resource.MustStructJSON(`
{
  "apiVersion": "some.example.com/v1alpha1",
  "kind": "Object",
  "metadata": {
    "name": "resource-name-b"
  },
  "status": {
    "conditions": [
      {
        "type": "Synced",
        "status": "True",
        "reason": "ReconcileSuccess"
      },
      {
        "type": "Ready",
        "status": "True",
        "reason": "Available"
      }
    ]
  }
}
`),
							},
							"resource-c": {
								Resource: resource.MustStructJSON(`
{
  "apiVersion": "some.example.com/v1alpha1",
  "kind": "Object",
  "metadata": {
    "name": "resource-name-c"
  },
  "status": {
    "conditions": [
      {
        "type": "Synced",
        "status": "True",
        "reason": "ReconcileSuccess"
      },
      {
        "type": "Ready",
        "status": "False",
        "reason": "Creating"
      }
    ]
  }
}
`),
							},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta: &fnv1beta1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1beta1.Condition{
						{
							Type:    "ShouldBeSet",
							Status:  fnv1beta1.Status_STATUS_CONDITION_TRUE,
							Reason:  "Testing",
							Message: ptr.To("This condition should be set. All resources match all conditions."),
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
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := &Function{log: logging.NewNopLogger()}
			rsp, err := f.RunFunction(tc.args.ctx, tc.args.req)

			// The function-sdk-go library depends on the go-json-experiment
			// library. The go-json-experiment library explicitly randomizes
			// their error messages between "cannot unmarshal..." and "unable
			// to unmarshal...". This is to "Hyrum-proof" (see Hyrum's Law)
			// their library. This means that we must un-Hyrum-proof the strings or
			// else our unit tests will occasionally fail.
			if tc.want.cleanError {
				for i := range rsp.GetConditions() {
					msg := rsp.GetConditions()[i].GetMessage()
					if msg == "" {
						continue
					}
					rsp.Conditions[i].Message = ptr.To(strings.ReplaceAll(msg, "unable to unmarshal Go value", "cannot unmarshal Go value"))
				}
			}

			if diff := cmp.Diff(tc.want.rsp, rsp, protocmp.Transform()); diff != "" {
				t.Errorf("%s\nf.RunFunction(...): -want rsp, +got rsp:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nf.RunFunction(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}
