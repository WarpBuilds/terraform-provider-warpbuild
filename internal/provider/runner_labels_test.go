package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestNormalizeLabels(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		labels []string
		runner string
		want   []string
	}{
		"appends runner name": {
			labels: []string{"dependabot", "code-scanning"},
			runner: "warp-custom-foo",
			want:   []string{"dependabot", "code-scanning", "warp-custom-foo"},
		},
		"lowercases": {
			labels: []string{"Dependabot", "CODE-SCANNING"},
			runner: "warp-custom-Foo",
			want:   []string{"dependabot", "code-scanning", "warp-custom-foo"},
		},
		"deduplicates": {
			labels: []string{"gpu", "gpu", "GPU"},
			runner: "warp-custom-foo",
			want:   []string{"gpu", "warp-custom-foo"},
		},
		"name already present": {
			labels: []string{"warp-custom-foo", "dependabot"},
			runner: "warp-custom-foo",
			want:   []string{"warp-custom-foo", "dependabot"},
		},
		"empty labels": {
			labels: nil,
			runner: "warp-custom-foo",
			want:   []string{"warp-custom-foo"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := normalizeLabels(tc.labels, tc.runner)
			if !sameLabelSet(got, tc.want) {
				t.Fatalf("normalizeLabels(%v, %q) = %v, want %v", tc.labels, tc.runner, got, tc.want)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("normalizeLabels(%v, %q) returned %d labels, want %d", tc.labels, tc.runner, len(got), len(tc.want))
			}
		})
	}
}

func TestNormalizeLabelsDoesNotMutateInput(t *testing.T) {
	t.Parallel()

	labels := []string{"Dependabot"}
	normalizeLabels(labels, "warp-custom-foo")
	if labels[0] != "Dependabot" {
		t.Fatalf("input was mutated: %v", labels)
	}
}

func TestSameLabelSet(t *testing.T) {
	t.Parallel()

	if !sameLabelSet([]string{"a", "b"}, []string{"b", "a"}) {
		t.Fatal("expected order to be irrelevant")
	}
	if sameLabelSet([]string{"a"}, []string{"a", "b"}) {
		t.Fatal("expected differing lengths to compare unequal")
	}
	if sameLabelSet([]string{"a", "b"}, []string{"a", "c"}) {
		t.Fatal("expected differing members to compare unequal")
	}
	if sameLabelSet([]string{"a", "b"}, []string{"a", "a"}) {
		t.Fatal("expected a repeated element to compare unequal despite matching length")
	}
}

func setOf(t *testing.T, values ...string) types.Set {
	t.Helper()
	set, diags := types.SetValueFrom(context.Background(), types.StringType, values)
	if diags.HasError() {
		t.Fatalf("building set: %v", diags)
	}
	return set
}

func labelsOf(t *testing.T, set types.Set) []string {
	t.Helper()
	var out []string
	if diags := set.ElementsAs(context.Background(), &out, false); diags.HasError() {
		t.Fatalf("reading set: %v", diags)
	}
	return out
}

func TestReconcileLabels(t *testing.T) {
	t.Parallel()

	const runner = "warp-custom-foo"

	for name, tc := range map[string]struct {
		prior types.Set
		api   []string
		want  []string
	}{
		"keeps configured set when API only added the name": {
			prior: setOf(t, "dependabot", "code-scanning"),
			api:   []string{"dependabot", "code-scanning", runner},
			want:  []string{"dependabot", "code-scanning"},
		},
		"keeps configured casing": {
			prior: setOf(t, "Dependabot"),
			api:   []string{"dependabot", runner},
			want:  []string{"Dependabot"},
		},
		"keeps configured set when the name was listed explicitly": {
			prior: setOf(t, runner, "dependabot"),
			api:   []string{runner, "dependabot"},
			want:  []string{runner, "dependabot"},
		},
		"takes API value when labels changed outside Terraform": {
			prior: setOf(t, "dependabot"),
			api:   []string{"dependabot", "added-in-ui", runner},
			want:  []string{"dependabot", "added-in-ui", runner},
		},
		"takes API value when prior is null": {
			prior: types.SetNull(types.StringType),
			api:   []string{runner},
			want:  []string{runner},
		},
		"takes API value when prior is unknown": {
			prior: types.SetUnknown(types.StringType),
			api:   []string{runner},
			want:  []string{runner},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, diags := reconcileLabels(context.Background(), tc.prior, tc.api, runner)
			if diags.HasError() {
				t.Fatalf("reconcileLabels: %v", diags)
			}
			gotLabels := labelsOf(t, got)
			if !sameLabelSet(gotLabels, tc.want) || len(gotLabels) != len(tc.want) {
				t.Fatalf("reconcileLabels = %v, want %v", gotLabels, tc.want)
			}
		})
	}
}
