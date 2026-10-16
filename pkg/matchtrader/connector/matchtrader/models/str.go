package models

import (
	"bytes"
	"encoding/json"
)

type Str string

func (s *Str) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		*s = ""
		return nil
	}
	if data[0] == '"' {
		var v string
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		*s = Str(v)
		return nil
	}
	*s = Str(data)
	return nil
}

func (s Str) String() string { return string(s) }
