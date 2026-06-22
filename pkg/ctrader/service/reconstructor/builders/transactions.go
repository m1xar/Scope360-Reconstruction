package builders

import (
	"math"
	"sort"

	pb "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/proto"
	"github.com/m1xar/scope360-reconstruction/pkg/ctrader/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
)

func BuildTransactions(cashFlow []*pb.ProtoOADepositWithdraw) []domain.Transaction {
	out := make([]domain.Transaction, 0, len(cashFlow))
	for _, item := range cashFlow {
		if item == nil {
			continue
		}

		var typ string
		switch item.GetOperationType() {
		case pb.ProtoOAChangeBalanceType_BALANCE_DEPOSIT,
			pb.ProtoOAChangeBalanceType_BALANCE_DEPOSIT_TRANSFER:
			typ = domain.TransactionTypeDeposit
		case pb.ProtoOAChangeBalanceType_BALANCE_WITHDRAW,
			pb.ProtoOAChangeBalanceType_BALANCE_WITHDRAW_TRANSFER:
			typ = domain.TransactionTypeWithdrawal
		default:
			continue
		}

		amount := helpers.Round8(math.Abs(helpers.Money(item.GetDelta(), item.GetMoneyDigits())))
		if amount == 0 {
			continue
		}
		out = append(out, domain.Transaction{
			Time:   helpers.TimeFromMillis(item.GetChangeBalanceTimestamp()),
			Type:   typ,
			Amount: amount,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Time.Before(out[j].Time)
	})
	return out
}
