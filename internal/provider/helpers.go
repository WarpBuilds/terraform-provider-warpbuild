package provider

import (
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func int32Ptr(v int64) *int32 {
	i := int32(v)
	return &i
}

func int64PtrValue(v *int32) types.Int64 {
	if v == nil {
		return types.Int64Null()
	}
	return types.Int64Value(int64(*v))
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
