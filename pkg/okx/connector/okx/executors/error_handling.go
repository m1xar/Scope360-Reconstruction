package executors

import (
	"errors"
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx"
)

const (
	rateLimitSleep   = 2 * time.Second
	rateLimitRetries = 3
)

func isHTTP5xx(err error) bool {
	var httpErr *okx.HTTPError
	return errors.As(err, &httpErr) && httpErr.StatusCode >= 500
}

func isHTTP429(err error) bool {
	var httpErr *okx.HTTPError
	return errors.As(err, &httpErr) && httpErr.StatusCode == 429
}

func doWithRateLimit[T any](fn func() (T, error)) (T, error) {
	for i := 0; ; i++ {
		result, err := fn()
		if err != nil && isHTTP429(err) && i < rateLimitRetries {
			time.Sleep(rateLimitSleep)
			continue
		}
		return result, err
	}
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
