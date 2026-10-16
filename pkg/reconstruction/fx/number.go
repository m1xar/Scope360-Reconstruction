package fx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type Number float64

func (n *Number) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		*n = 0
		return nil
	}
	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		s = strings.TrimSpace(s)
		if s == "" {
			*n = 0
			return nil
		}
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return fmt.Errorf("fx.Number: %q: %w", s, err)
		}
		*n = Number(v)
		return nil
	}
	var v float64
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*n = Number(v)
	return nil
}

func (n Number) MarshalJSON() ([]byte, error) { return json.Marshal(float64(n)) }

func (n Number) Float() float64 { return float64(n) }
