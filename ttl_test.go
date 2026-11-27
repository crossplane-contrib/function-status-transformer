package main

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/response"
)

func TestRunFunctionTTL(t *testing.T) {
	f := &Function{log: logging.NewNopLogger()}
	req := &fnv1.RunFunctionRequest{
		Input: resource.MustStructJSON(`{"statusConditionHooks": []}`),
		Observed: &fnv1.State{Composite: &fnv1.Resource{
			Resource: resource.MustStructJSON(`{"apiVersion":"example.org/v1","kind":"XR","metadata":{"name":"example"}}`),
		}},
		Desired: &fnv1.State{Resources: map[string]*fnv1.Resource{
			"child": {Resource: resource.MustStructJSON(`{"apiVersion":"v1","kind":"ConfigMap","data":{"key":"value"}}`)},
		}},
	}
	baseline, err := f.RunFunction(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		ttl   string
		want  time.Duration
		fatal string
	}{
		"Default":  {want: response.DefaultTTL},
		"Custom":   {ttl: "5m", want: 5 * time.Minute},
		"Zero":     {ttl: "0s"},
		"Invalid":  {ttl: "five minutes", fatal: `cannot parse ttl: time: invalid duration "five minutes"`},
		"Negative": {ttl: "-1s", fatal: "ttl duration cannot be negative"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			request := proto.Clone(req).(*fnv1.RunFunctionRequest)
			request.Input.Fields["ttl"] = structpb.NewStringValue(tc.ttl)
			got, err := f.RunFunction(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			if tc.fatal != "" {
				if len(got.GetResults()) != 1 || got.GetResults()[0].GetSeverity() != fnv1.Severity_SEVERITY_FATAL || got.GetResults()[0].GetMessage() != tc.fatal {
					t.Fatalf("expected fatal result %q, got %v", tc.fatal, got.GetResults())
				}
				return
			}
			want := proto.Clone(baseline).(*fnv1.RunFunctionResponse)
			want.Meta.Ttl = durationpb.New(tc.want)
			if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
				t.Errorf("response changed beyond its TTL (-want +got):\n%s", diff)
			}
		})
	}
}
