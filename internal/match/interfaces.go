package match

import (
	"github.com/reconcile/internal/model"
	"github.com/reconcile/internal/rules"
)

// Matcher runs one pass of the reconciliation algorithm against a slice of records.
// It returns matched groups and any records it could not match in this pass.
type Matcher interface {
	Match(records []model.Record, sources []string, cfg rules.MatchingConfig) (matches []model.MatchResult, unmatched []model.Record)
}

// ExceptionClassifier assigns a typed reason code to every record that
// was not matched by any Matcher.
type ExceptionClassifier interface {
	Classify(unmatched []model.Record, all []model.Record, sources []string, cfg rules.MatchingConfig) []model.Exception
}

// exactMatcher implements Matcher using deterministic key-based exact matching.
type exactMatcher struct{}

// NewExactMatcher returns a Matcher that performs the exact-key pass.
func NewExactMatcher() Matcher { return &exactMatcher{} }

func (e *exactMatcher) Match(records []model.Record, sources []string, cfg rules.MatchingConfig) ([]model.MatchResult, []model.Record) {
	matches, unmatched, _ := ExactMatchPass(records, sources, cfg.Exact)
	return matches, unmatched
}

// fuzzyMatcher implements Matcher using tolerance-based fuzzy matching.
type fuzzyMatcher struct{}

// NewFuzzyMatcher returns a Matcher that performs the fuzzy/graph pass.
func NewFuzzyMatcher() Matcher { return &fuzzyMatcher{} }

func (f *fuzzyMatcher) Match(records []model.Record, sources []string, cfg rules.MatchingConfig) ([]model.MatchResult, []model.Record) {
	return FuzzyMatchPass(records, sources, cfg.Fuzzy)
}

// ruleClassifier implements ExceptionClassifier using typed rule-based classification.
type ruleClassifier struct{}

// NewRuleClassifier returns an ExceptionClassifier that uses the built-in rule set.
func NewRuleClassifier() ExceptionClassifier { return &ruleClassifier{} }

func (r *ruleClassifier) Classify(unmatched []model.Record, all []model.Record, sources []string, cfg rules.MatchingConfig) []model.Exception {
	return ClassifyExceptions(unmatched, all, sources, cfg)
}
