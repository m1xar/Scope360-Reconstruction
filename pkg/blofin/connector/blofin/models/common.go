package models

import (
	"strconv"
	"time"
)

func Float(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func Int64(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

func TimeFromMs(ms string) time.Time {
	return time.UnixMilli(Int64(ms)).UTC()
}
