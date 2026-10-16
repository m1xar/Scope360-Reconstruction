package models

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var layouts = []string{
	time.RFC3339Nano,
	"2006-01-02T15:04:05.999999999",
	"2006-01-02 15:04:05.999999999",
	"2006-01-02T15:04",
	"2006-01-02",
}

func ParseTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
	}
	if n, err := strconv.ParseFloat(s, 64); err == nil {
		return FromEpoch(n), nil
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("matchtrader: unknown time format %q", s)
}

func FromEpoch(n float64) time.Time {
	if n <= 0 {
		return time.Time{}
	}
	if n < 1e11 {
		return time.Unix(0, int64(n*1e9)).UTC()
	}
	return time.UnixMilli(int64(n)).UTC()
}

type Time struct{ time.Time }

func (t *Time) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		t.Time = time.Time{}
		return nil
	}
	var s string
	if data[0] == '"' {
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
	} else {
		s = string(data)
	}
	parsed, err := ParseTime(s)
	if err != nil {
		return err
	}
	t.Time = parsed
	return nil
}

func Pick(millis float64, t Time) time.Time {
	if millis > 0 {
		return FromEpoch(millis)
	}
	return t.Time
}

func FormatTime(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05.000Z")
}
