package executors

import (
	"errors"

	blofin "github.com/m1xar/scope360-reconstruction/pkg/blofin/connector/blofin"
)

func isHTTP5xx(err error) bool {
	var httpErr *blofin.HTTPError
	return errors.As(err, &httpErr) && httpErr.StatusCode >= 500
}
