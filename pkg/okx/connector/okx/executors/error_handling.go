package executors

import (
	"errors"

	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/okx/connector/okx"
)

func isHTTP5xx(err error) bool {
	var httpErr *okx.HTTPError
	return errors.As(err, &httpErr) && httpErr.StatusCode >= 500
}

func mergeInstTypeResults[T any](swapData []T, swapErr error, futuresData []T, futuresErr error) ([]T, error) {
	if swapErr == nil && futuresErr == nil {
		return append(swapData, futuresData...), nil
	}

	if swapErr == nil && futuresErr != nil {
		if isHTTP5xx(futuresErr) {
			return swapData, nil
		}
		return nil, futuresErr
	}

	if swapErr != nil && futuresErr == nil {
		if isHTTP5xx(swapErr) {
			return futuresData, nil
		}
		return nil, swapErr
	}

	if isHTTP5xx(swapErr) && isHTTP5xx(futuresErr) {
		return nil, swapErr
	}

	if !isHTTP5xx(swapErr) {
		return nil, swapErr
	}
	return nil, futuresErr
}
