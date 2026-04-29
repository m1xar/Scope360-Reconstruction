package executors

import "time"

func ParseKrakenTime(raw string) (time.Time, error) {
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
	}
	var lastErr error
	for _, layout := range layouts {
		t, err := time.Parse(layout, raw)
		if err == nil {
			return t.UTC(), nil
		}
		lastErr = err
	}
	return time.Time{}, lastErr
}

func FormatKrakenTime(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05.000Z")
}
