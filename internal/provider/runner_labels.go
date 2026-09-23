package provider

import (
	"context"
	"maps"
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// like the api, the runner name is appended, every label is lowercased,
// and the result is deduplicated.
func normalizeLabels(labels []string, name string) []string {
	seen := make(map[string]struct{}, len(labels)+1)
	for _, label := range slices.Concat(labels, []string{name}) {
		seen[strings.ToLower(label)] = struct{}{}
	}
	return slices.Sorted(maps.Keys(seen))
}

func sameLabelSet(a, b []string) bool {
	return slices.Equal(slices.Sorted(slices.Values(a)), slices.Sorted(slices.Values(b)))
}

// reconcileLabels decides what to store for `labels`. Terraform requires the
// stored value to equal the configured one, but the API answers with its
// normalized set, so the configured value is kept whenever the API's set is
// what normalization would produce from it. Anything else is a real change
// made outside Terraform and is stored as returned.
func reconcileLabels(ctx context.Context, prior types.Set, apiLabels []string, name string) (types.Set, diag.Diagnostics) {
	var diags diag.Diagnostics

	if !prior.IsNull() && !prior.IsUnknown() {
		var priorLabels []string
		diags.Append(prior.ElementsAs(ctx, &priorLabels, false)...)
		if diags.HasError() {
			return types.SetNull(types.StringType), diags
		}
		if sameLabelSet(normalizeLabels(priorLabels, name), apiLabels) {
			return prior, diags
		}
	}

	return types.SetValueFrom(ctx, types.StringType, apiLabels)
}
