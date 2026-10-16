package matchtrader

import (
	"bytes"
	"encoding/json"
	"strings"
)

const redacted = "<redacted>"

var secretKeys = map[string]bool{
	"password":        true,
	"token":           true,
	"tradingapitoken": true,
	"tradingtoken":    true,
	"rt":              true,
	"refresh":         true,
	"refreshtoken":    true,
	"accesstoken":     true,
	"co-auth":         true,
}

func Redact(body []byte) []byte {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || (trimmed[0] != '{' && trimmed[0] != '[') {
		return body
	}
	var v any
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return body
	}
	out, err := json.Marshal(redactValue(v))
	if err != nil {
		return body
	}
	return out
}

func redactValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			key := strings.ToLower(k)
			switch {
			case key == "tradingaccounttoken":
				if _, ok := val.(map[string]any); !ok && val != nil {
					t[k] = redacted
					continue
				}
				t[k] = redactValue(val)
			case secretKeys[key] && val != nil:
				t[k] = redacted
			default:
				t[k] = redactValue(val)
			}
		}
		return t
	case []any:
		for i := range t {
			t[i] = redactValue(t[i])
		}
		return t
	default:
		return v
	}
}

func (c *Client) record(e Exchange) {
	if c.recorder == nil {
		return
	}
	e.RequestBody = Redact(e.RequestBody)
	e.ResponseBody = Redact(e.ResponseBody)
	c.recorder(e)
}
