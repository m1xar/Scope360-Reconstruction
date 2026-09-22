package scope

type Scope uint8

const (
	Closed Scope = 1 << iota
	Open
	Balances
	Transactions
	Fundings

	All = Closed | Open | Balances | Transactions | Fundings
)

func (s Scope) Has(x Scope) bool { return s&x == x }

func (s Scope) Any(x Scope) bool { return s&x != 0 }
