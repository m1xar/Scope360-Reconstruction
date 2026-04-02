package executors

import (
	"errors"

	mexc "github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc"
)

func isHTTP5xx(err error) bool {
	var httpErr *mexc.HTTPError
	return errors.As(err, &httpErr) && httpErr.StatusCode >= 500
}
