package models

type OrderlyResponse[T any] struct {
	Success   bool  `json:"success"`
	Timestamp int64 `json:"timestamp"`
	Data      T     `json:"data"`
}

type PagedData[T any] struct {
	Meta PageMeta `json:"meta"`
	Rows []T      `json:"rows"`
}

type PageMeta struct {
	Total          int `json:"total"`
	RecordsPerPage int `json:"records_per_page"`
	CurrentPage    int `json:"current_page"`
}
