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
	inputCRD    = "../../package/input/function-status-transformer.fn.crossplane.io_statustransformations.yaml"
	enumMarker  = "+kubebuilder:validation:Enum="
)

// TestMatchTypeValuesInSync verifies that every representation of the MatchType
// values agrees: the Go constants, the kubebuilder enum marker that drives
// code generation, and the generated input CRD shipped in the package. These
// drifted apart in the past, which let users write a `type` that passed CRD
// validation but that the function did not recognise. Regenerating the input
// CRD (go generate ./input/...) is what keeps them aligned; this test fails
// when one of them changes without the others.
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

	t.Run("GeneratedCRD", func(t *testing.T) {
		doc, err := os.ReadFile(inputCRD)
		if err != nil {
			t.Fatalf("cannot read generated CRD: %v", err)
		}
		got := firstEnumBlock(t, string(doc))
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("generated CRD MatchType enum is out of sync with the constants (-want +got):\n%s", diff)
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

// firstEnumBlock returns the sorted values of the first YAML `enum:` block in
// the given document.
func firstEnumBlock(t *testing.T, doc string) []string {
	t.Helper()
	lines := strings.Split(doc, "\n")

	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "enum:" {
			start = i
			break
		}
	}
	if start == -1 {
		t.Fatal("no enum block found in the generated CRD")
	}

	var vals []string
	for _, line := range lines[start+1:] {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "- ") {
			break
		}
		vals = append(vals, strings.TrimPrefix(trimmed, "- "))
	}
	sort.Strings(vals)
	return vals
}
