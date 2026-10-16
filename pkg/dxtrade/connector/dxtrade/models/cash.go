package models

import (
	"fmt"

	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/fx"
)

const (
	CashCommission = "COMMISSION"
	CashFinancing  = "FINANCING"
	CashDeposit    = "DEPOSIT"
	CashWithdrawal = "WITHDRAWAL"
	CashSettlement = "SETTLEMENT"
	CashCost       = "COST"
	CashDividend   = "EX_DIVIDEND"
	CashRebate     = "REBATE"
)

type CashTransaction struct {
	Account         string    `json:"account"`
	TransactionCode ID        `json:"transactionCode"`
	OrderCode       string    `json:"orderCode"`
	TradeCode       ID        `json:"tradeCode"`
	PositionCode    ID        `json:"positionCode"`
	OrderID         ID        `json:"orderId"`
	Version         fx.Number `json:"version"`
	ClientOrderID   string    `json:"clientOrderId"`
	Type            string    `json:"type"`
	Value           fx.Number `json:"value"`
	Currency        string    `json:"currency"`
	TransactionTime Time      `json:"transactionTime"`
	CreatedTime     Time      `json:"createdTime"`
}

type CashTransferList struct {
	CashTransfers           []CashTransfer `json:"cashTransfers"`
	NextPageTransactionTime Time           `json:"nextPageTransactionTime"`
}

type CashTransfer struct {
	Account          string            `json:"account"`
	Version          fx.Number         `json:"version"`
	TransferCode     string            `json:"transferCode"`
	Comment          string            `json:"comment"`
	TransactionTime  Time              `json:"transactionTime"`
	CreatedTime      Time              `json:"createdTime"`
	CashTransactions []CashTransaction `json:"cashTransactions"`
}

func (tx CashTransaction) Key() string {
	if code := tx.TransactionCode.String(); code != "" {
		return code + "|" + tx.Type
	}
	return fmt.Sprintf("%s|%s|%s|%v|%s", tx.Type, tx.TransactionTime.Raw, tx.OrderCode, float64(tx.Value), tx.PositionCode)
}
