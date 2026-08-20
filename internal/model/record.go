package model

import "time"

type Record struct {
	ID          string    // source-native ID
	Source      string    // "gateway" | "bank" | "ledger"
	RefID       string    // transaction/reference ID — primary join key
	Amount      float64
	Currency    string
	Date        time.Time
	Description string
}

type MatchResult struct {
	MatchID     string
	Records     []Record       // 2-3 matched records across sources
	Pass        string         // "exact" | "fuzzy"
	RuleApplied string         // which rule from rules.yaml fired
	Confidence  float64        // 1.0 for exact, tolerance-derived for fuzzy
}

type Exception struct {
	Record     Record
	ReasonCode string // AMOUNT_MISMATCH | NO_COUNTERPART | DATE_DRIFT | DUPLICATE_REF | MALFORMED_INPUT
	Detail     string // human-readable explanation
}
