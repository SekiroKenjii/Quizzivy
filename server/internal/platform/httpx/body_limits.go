package httpx

import (
	"encoding/json"
	"github.com/getkin/kin-openapi/openapi3"
)

// RequestBodyLimits reads positive integral byte budgets from contract operations; absent or invalid overrides retain the default limit.
func RequestBodyLimits(spec *openapi3.T) map[string]int64 {
	limits := map[string]int64{}
	for path, item := range spec.Paths.Map() {
		for method, operation := range item.Operations() {
			if operation == nil {
				continue
			}
			if limit := bodyLimit(operation.Extensions["x-max-body-bytes"]); limit > 0 {
				limits[method+" "+path] = limit
			}
		}
	}
	return limits
}

func bodyLimit(raw any) int64 {
	encoded, err := json.Marshal(raw)
	if err != nil {
		return 0
	}
	var limit int64
	if err := json.Unmarshal(encoded, &limit); err != nil {
		return 0
	}
	return limit
}
