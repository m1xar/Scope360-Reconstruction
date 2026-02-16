package models

type L2Book struct {
	Coin   string      `json:"coin"`
	Time   int64       `json:"time"`
	Levels [][]L2Level `json:"levels"`
}

type L2Level struct {
	Px string `json:"px"`
	Sz string `json:"sz"`
	N  int    `json:"n"`
}
