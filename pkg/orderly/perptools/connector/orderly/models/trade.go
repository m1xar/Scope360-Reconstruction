package models

import (
	"encoding/json"
	"fmt"
	"strings"
)

type OrderlyTrade struct {
	ID                int     `json:"id"`
	Symbol            string  `json:"symbol"`
	Fee               float64 `json:"fee"`
	FeeAsset          string  `json:"fee_asset"`
	Side              string  `json:"side"`
	OrderID           int64   `json:"order_id"`
	ExecutedPrice     float64 `json:"executed_price"`
	ExecutedQuantity  float64 `json:"executed_quantity"`
	ExecutedTimestamp int64   `json:"executed_timestamp"`
	IsMaker           bool    `json:"is_maker"`
	RealizedPnl       float64 `json:"realized_pnl"`
	MatchID           int64   `json:"match_id"`
	MarginToken       string  `json:"margin_token"`
	MarginChangeQty   float64 `json:"margin_change_qty"`
	MarginMode        string  `json:"margin_mode"`
}

func (t *OrderlyTrade) UnmarshalJSON(data []byte) error {
	type alias struct {
		ID                int             `json:"id"`
		Symbol            string          `json:"symbol"`
		Fee               float64         `json:"fee"`
		FeeAsset          string          `json:"fee_asset"`
		Side              string          `json:"side"`
		OrderID           int64           `json:"order_id"`
		ExecutedPrice     float64         `json:"executed_price"`
		ExecutedQuantity  float64         `json:"executed_quantity"`
		ExecutedTimestamp int64           `json:"executed_timestamp"`
		IsMakerRaw        json.RawMessage `json:"is_maker"`
		RealizedPnl       float64         `json:"realized_pnl"`
		MatchID           int64           `json:"match_id"`
		MarginToken       string          `json:"margin_token"`
		MarginChangeQty   float64         `json:"margin_change_qty"`
		MarginMode        string          `json:"margin_mode"`
	}

	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}

	isMaker, err := parseMakerFlag(a.IsMakerRaw)
	if err != nil {
		return fmt.Errorf("invalid is_maker: %w", err)
	}

	*t = OrderlyTrade{
		ID:                a.ID,
		Symbol:            a.Symbol,
		Fee:               a.Fee,
		FeeAsset:          a.FeeAsset,
		Side:              a.Side,
		OrderID:           a.OrderID,
		ExecutedPrice:     a.ExecutedPrice,
		ExecutedQuantity:  a.ExecutedQuantity,
		ExecutedTimestamp: a.ExecutedTimestamp,
		IsMaker:           isMaker,
		RealizedPnl:       a.RealizedPnl,
		MatchID:           a.MatchID,
		MarginToken:       a.MarginToken,
		MarginChangeQty:   a.MarginChangeQty,
		MarginMode:        a.MarginMode,
	}

	return nil
}

func parseMakerFlag(raw json.RawMessage) (bool, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return false, nil
	}

	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		return b, nil
	}

	var n int
	if err := json.Unmarshal(raw, &n); err == nil {
		if n == 0 {
			return false, nil
		}
		if n == 1 {
			return true, nil
		}
		return false, fmt.Errorf("numeric value must be 0 or 1, got %d", n)
	}

	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		switch strings.ToLower(strings.TrimSpace(s)) {
		case "true", "1":
			return true, nil
		case "false", "0":
			return false, nil
		default:
			return false, fmt.Errorf("string value must be true/false/0/1, got %q", s)
		}
	}

	return false, fmt.Errorf("unsupported JSON type: %s", string(raw))
}
