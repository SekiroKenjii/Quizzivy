package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/getkin/kin-openapi/openapi3"
	"math"
	"strconv"
)

// RequestBodyLimit is a contract byte budget and its oversized-response status.
type RequestBodyLimit struct {
	Bytes          int64
	ExceededStatus int
}

// RequestBodyLimits resolves contract budgets and rejects invalid new status metadata.
func RequestBodyLimits(spec *openapi3.T) (map[string]RequestBodyLimit, error) {
	limits := map[string]RequestBodyLimit{}
	streaming := StreamingBodyRoutes(spec)
	for path, item := range spec.Paths.Map() {
		for method, operation := range item.Operations() {
			if operation == nil {
				continue
			}
			route := method + " " + path
			_, streams := streaming[route]
			limit, err := operationBodyLimit(operation, streams)
			if err != nil {
				return nil, fmt.Errorf("%s body limit: %w", route, err)
			}
			if limit.Bytes > 0 {
				limits[route] = limit
			}
		}
	}
	return limits, nil
}

func operationBodyLimit(op *openapi3.Operation, streaming bool) (RequestBodyLimit, error) {
	limit := RequestBodyLimit{Bytes: bodyLimit(op.Extensions["x-max-body-bytes"]), ExceededStatus: 413}
	raw, specified := op.Extensions["x-body-limit-status"]
	if !specified {
		return limit, nil
	}
	status := bodyLimit(raw)
	if status != 400 && status != 413 {
		return limit, errors.New("x-body-limit-status must be integral 400 or 413")
	}
	if limit.Bytes <= 0 || limit.Bytes == math.MaxInt64 {
		return limit, errors.New("x-body-limit-status requires a positive bounded x-max-body-bytes")
	}
	if op.RequestBody == nil || op.RequestBody.Value == nil {
		return limit, errors.New("x-body-limit-status requires a resolved request body")
	}
	if streaming {
		return limit, errors.New("x-body-limit-status cannot select streaming errors")
	}
	if op.Responses == nil || op.Responses.Value(strconv.Itoa(int(status))) == nil {
		return limit, errors.New("x-body-limit-status requires its explicit response")
	}
	limit.ExceededStatus = int(status)
	return limit, nil
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
