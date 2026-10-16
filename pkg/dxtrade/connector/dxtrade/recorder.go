package dxtrade

import (
	"regexp"
	"time"
)

type Exchange struct {
	Name         string
	Method       string
	Path         string
	RequestBody  []byte
	Status       int
	ResponseBody []byte
	Duration     time.Duration
	Err          error
}

type Recorder func(Exchange)

const redacted = "<redacted>"

var secretFields = regexp.MustCompile(`("(?:password|sessionToken|token)"\s*:\s*)"(?:[^"\\]|\\.)*"`)

func Redact(body []byte) []byte {
	if len(body) == 0 {
		return body
	}
	return secretFields.ReplaceAll(body, []byte(`${1}"`+redacted+`"`))
}

func (c *Client) record(ex Exchange) {
	if c.cfg.Recorder == nil {
		return
	}
	ex.RequestBody = Redact(ex.RequestBody)
	ex.ResponseBody = Redact(ex.ResponseBody)
	c.cfg.Recorder(ex)
}
