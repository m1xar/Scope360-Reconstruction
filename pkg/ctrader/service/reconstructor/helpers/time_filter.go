package helpers

import "time"

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
	if days > 0 {
		return to.AddDate(0, 0, -days), to
	}
	return time.Unix(0, 0), to
}
