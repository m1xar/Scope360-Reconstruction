package models

import (
	"strings"
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/fx"
)

const (
	FinanceDeposit         = "DEPOSIT"
	FinanceWithdrawal      = "WITHDRAWAL"
	FinanceCreditIn        = "CREDIT_IN"
	FinanceCreditOut       = "CREDIT_OUT"
	FinanceAgentCommission = "AGENT_COMMISSION"
	FinanceOther           = "OTHER"
)

var FinanceTypes = []int{0, 1, 2, 3, 4, 5}

var financeNames = []string{FinanceDeposit, FinanceWithdrawal, FinanceCreditIn, FinanceCreditOut, FinanceAgentCommission, FinanceOther}

type FinanceOperation struct {
	ID         Str       `json:"id"`
	Type       Str       `json:"type"`
	Time       Time      `json:"time"`
	TimeMillis fx.Number `json:"timeMillis"`
	Profit     fx.Number `json:"profit"`
}

func (f FinanceOperation) At() time.Time { return Pick(f.TimeMillis.Float(), f.Time) }

func (f FinanceOperation) Kind() string {
	t := strings.ToUpper(strings.TrimSpace(string(f.Type)))
	if len(t) == 1 && t[0] >= '0' && int(t[0]-'0') < len(financeNames) {
		return financeNames[t[0]-'0']
	}
	return t
}
