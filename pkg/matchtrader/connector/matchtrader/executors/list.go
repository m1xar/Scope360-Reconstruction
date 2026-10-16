package executors

import (
	"bytes"
	"encoding/json"
	"fmt"
)

func decodeList[T any](name string, raw json.RawMessage, keys ...string) ([]T, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil, nil
	}
	var out []T
	if raw[0] == '[' {
		if err := json.Unmarshal(raw, &out); err != nil {
			return nil, fmt.Errorf("matchtrader %s: decode: %w", name, err)
		}
		return out, nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("matchtrader %s: decode: %w", name, err)
	}
	for _, k := range keys {
		if v, ok := obj[k]; ok {
			return decodeList[T](name, v)
		}
	}
	return nil, nil
}
