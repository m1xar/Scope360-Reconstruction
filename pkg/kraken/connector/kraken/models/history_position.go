package models

type PositionEventsResponse struct {
	Elements          []PositionEventElement `json:"elements"`
	ContinuationToken string                 `json:"continuationToken"`
}

type PositionEventElement struct {
	UID       string                `json:"uid"`
	Timestamp int64                 `json:"timestamp"`
	Event     PositionEventEnvelope `json:"event"`
}

type PositionEventEnvelope struct {
	PositionUpdate PositionUpdate `json:"PositionUpdate"`
}

type PositionUpdate struct {
	AccountUID             string        `json:"accountUid"`
	Tradeable              string        `json:"tradeable"`
	OldPosition            FlexibleFloat `json:"oldPosition"`
	OldAverageEntryPrice   FlexibleFloat `json:"oldAverageEntryPrice"`
	NewPosition            FlexibleFloat `json:"newPosition"`
	NewAverageEntryPrice   FlexibleFloat `json:"newAverageEntryPrice"`
	FillTime               int64         `json:"fillTime"`
	Fee                    FlexibleFloat `json:"fee"`
	FeeCurrency            string        `json:"feeCurrency"`
	RealizedPnL            FlexibleFloat `json:"realizedPnL"`
	PositionChange         string        `json:"positionChange"`
	ExecutionUID           string        `json:"executionUid"`
	ExecutionPrice         FlexibleFloat `json:"executionPrice"`
	ExecutionSize          FlexibleFloat `json:"executionSize"`
	TradeType              string        `json:"tradeType"`
	FundingRealizationTime int64         `json:"fundingRealizationTime"`
	RealizedFunding        FlexibleFloat `json:"realizedFunding"`
	Timestamp              int64         `json:"timestamp"`
	UpdateReason           string        `json:"updateReason"`
}
