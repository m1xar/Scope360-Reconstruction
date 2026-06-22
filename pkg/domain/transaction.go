package domain

import "time"

const (
	TransactionTypeDeposit    = "DEPOSIT"
	TransactionTypeWithdrawal = "WITHDRAWAL"
)

type Transaction struct {
	Time   time.Time
	Type   string
	Amount float64
}
