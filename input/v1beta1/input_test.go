// SPDX-FileCopyrightText: 2025 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

const (
	inputSource = "input.go"
	enumMarker  = "+kubebuilder:validation:Enum="
)

// TestMatchTypeValuesInSync verifies that the kubebuilder enum marker that
// drives code generation lists exactly the Go MatchType constants. These
// drifted apart in the past, which let users write a `type` that passed CRD
// validation but that the function did not recognise. controller-gen doesn't
// derive the marker's values from the constants, so this test fails when one
// changes without the other. CI separately checks that the generated input CRD
// matches the marker.
func TestMatchTypeValuesInSync(t *testing.T) {
	want := []string{
		string(AnyResourceMatchesAnyCondition),
		string(AnyResourceMatchesAllConditions),
		string(AllResourcesMatchAnyCondition),
		string(AllResourcesMatchAllConditions),
	}
	sort.Strings(want)

	t.Run("Marker", func(t *testing.T) {
		got, err := enumMarkerValues(inputSource)
		if err != nil {
			t.Fatalf("cannot read %s: %v", inputSource, err)
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("MatchType enum marker is out of sync with the constants (-want +got):\n%s", diff)
		}
	})
}

// enumMarkerValues returns the sorted values of the kubebuilder enum marker
// found in the given Go source file.
func enumMarkerValues(path string) ([]string, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(string(src), "\n") {
		i := strings.Index(line, enumMarker)
		if i < 0 {
			continue
		}
		vals := strings.Split(strings.TrimSpace(line[i+len(enumMarker):]), ";")
		sort.Strings(vals)
		return vals, nil
	}
	return nil, nil
}
