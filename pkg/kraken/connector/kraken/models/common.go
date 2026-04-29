package models

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type FlexibleFloat float64

func (f *FlexibleFloat) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		*f = 0
		return nil
	}

	var n float64
	if err := json.Unmarshal(data, &n); err == nil {
		*f = FlexibleFloat(n)
		return nil
	}

	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if strings.TrimSpace(s) == "" {
		*f = 0
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fmt.Errorf("parse flexible float %q: %w", s, err)
	}
	*f = FlexibleFloat(v)
	return nil
}

func (f FlexibleFloat) Float64() float64 {
	return float64(f)
}

type NullableFloat struct {
	Value float64
	Valid bool
}

func (f *NullableFloat) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		f.Value = 0
		f.Valid = false
		return nil
	}

	var flex FlexibleFloat
	if err := json.Unmarshal(data, &flex); err != nil {
		return err
	}
	f.Value = flex.Float64()
	f.Valid = true
	return nil
}

type ResultResponse struct {
	Result     string `json:"result"`
	Error      string `json:"error"`
	ServerTime string `json:"serverTime"`
}
