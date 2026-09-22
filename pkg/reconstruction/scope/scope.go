// Package scope names the outputs a reconstruction load has to serve, so the
// loader can skip datasets nobody asked for and widen the ranges of the ones
// several consumers share.
package scope

type Scope uint8

const (
	Closed       Scope = 1 << iota // closed positions
	Open                           // open positions
	Balances                       // balance snapshots and the current balance
	Transactions                   // deposits and withdrawals
	Fundings                       // funding payments

	All = Closed | Open | Balances | Transactions | Fundings
)

// Has reports whether every bit of s is set in the scope.
func (s Scope) Has(x Scope) bool { return s&x == x }

// Any reports whether at least one bit of x is set in the scope.
func (s Scope) Any(x Scope) bool { return s&x != 0 }
