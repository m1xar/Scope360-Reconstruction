package models

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

type ID string

func (id *ID) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		*id = ""
		return nil
	}
	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*id = ID(s)
		return nil
	}
	*id = ID(data)
	return nil
}

func (id ID) String() string { return string(id) }

type Bool bool

func (b *Bool) UnmarshalJSON(data []byte) error {
	s := strings.Trim(strings.TrimSpace(string(data)), `"`)
	*b = Bool(strings.EqualFold(s, "true"))
	return nil
}

type Time struct {
	time.Time
	Raw string
}

var timeLayouts = []string{
	time.RFC3339Nano,
	"2006-01-02T15:04:05.999999999",
	"2006-01-02T15:04:05.999999999Z0700",
	"2006-01-02Z07:00",
	"2006-01-02",
}

func (t *Time) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	*t = Time{}
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		return nil
	}
	if data[0] != '"' {
		ms, err := strconv.ParseInt(string(data), 10, 64)
		if err != nil {
			return err
		}
		t.Raw = string(data)
		t.Time = time.UnixMilli(ms).UTC()
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	t.Raw = s
	t.Time = ParseTime(s)
	return nil
}

func (t Time) MarshalJSON() ([]byte, error) {
	if t.Raw != "" {
		return json.Marshal(t.Raw)
	}
	if t.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(FormatTime(t.Time))
}

func ParseTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	for _, layout := range timeLayouts {
		if v, err := time.Parse(layout, s); err == nil {
			return v.UTC()
		}
	}
	return time.Time{}
}

func FormatTime(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05.000Z")
}
