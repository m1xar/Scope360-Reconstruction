package helpers

import "time"

const DefaultHistoryDays = 365

func CutoffFromDays(days int) *time.Time {
	if days <= 0 {
		return nil
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	return &cutoff
}

func TimeFromMillis(ms int64) time.Time {
	if ms <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms).UTC()
}

func HistoryRange(days int) (time.Time, time.Time) {
	to := time.Now()
	if days <= 0 {
		days = DefaultHistoryDays
	}
	return to.AddDate(0, 0, -days), to
}
