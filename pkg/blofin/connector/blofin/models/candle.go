package models

import (
	"encoding/json"
	"fmt"
)

type Candle struct {
	Ts      string
	O       string
	H       string
	L       string
	C       string
	Vol     string
	VolCcy  string
	VolCcyQ string
	Confirm string
}

func (c *Candle) UnmarshalJSON(data []byte) error {
	var row []string
	if err := json.Unmarshal(data, &row); err != nil {
		return err
	}
	if len(row) < 6 {
		return fmt.Errorf("blofin candle: expected at least 6 columns, got %d", len(row))
	}
	c.Ts = row[0]
	c.O = row[1]
	c.H = row[2]
	c.L = row[3]
	c.C = row[4]
	c.Vol = row[5]
	if len(row) > 6 {
		c.VolCcy = row[6]
	}
	if len(row) > 7 {
		c.VolCcyQ = row[7]
	}
	if len(row) > 8 {
		c.Confirm = row[8]
	}
	return nil
}
