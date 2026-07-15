package provider

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func int32Ptr(v int64) *int32 {
	i := int32(v)
	return &i
}

// emptyStringAsNull maps both nil and "" to null. The API stores optional
// strings as zero values, so an unset field comes back as "" — which must not
// surface as a change when the practitioner never configured the attribute.
func emptyStringAsNull(s *string) types.String {
	if s == nil || *s == "" {
		return types.StringNull()
	}
	return types.StringValue(*s)
}

func int64PtrValue(v *int32) types.Int64 {
	if v == nil {
		return types.Int64Null()
	}
	return types.Int64Value(int64(*v))
}

// isNotFound reports whether an API error means the resource no longer
// exists. The WarpBuild API signals this as HTTP 400 with sub_code FVE_004
// ("no record found") rather than HTTP 404, so both are checked.
func isNotFound(httpResp *http.Response, err error) bool {
	if httpResp == nil {
		return false
	}
	if httpResp.StatusCode == http.StatusNotFound {
		return true
	}
	if apiErr, ok := err.(interface{ Body() []byte }); ok {
		return httpResp.StatusCode == http.StatusBadRequest &&
			strings.Contains(string(apiErr.Body()), `"FVE_004"`)
	}
	return false
}

// apiError formats a client error together with the API's response body,
// which carries WarpBuild's structured error message.
func apiError(httpResp *http.Response, err error) string {
	status := ""
	if httpResp != nil {
		status = fmt.Sprintf(" (HTTP %d)", httpResp.StatusCode)
	}
	// The generated client's GenericOpenAPIError keeps the response body.
	if apiErr, ok := err.(interface{ Body() []byte }); ok && len(apiErr.Body()) > 0 {
		return fmt.Sprintf("%s%s: %s", err.Error(), status, string(apiErr.Body()))
	}
	return err.Error() + status
}
