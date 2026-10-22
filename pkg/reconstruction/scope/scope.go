package scope

type Scope uint8

const (
	Closed Scope = 1 << iota
	Open
	Balances
	Transactions
	Fundings
	Spot

	All = Closed | Open | Balances | Transactions | Fundings | Spot
)

func (s Scope) Has(x Scope) bool { return s&x == x }

func (s Scope) Any(x Scope) bool { return s&x != 0 }
