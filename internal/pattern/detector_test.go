package pattern_test

import (
	"testing"

	"github.com/reconcile/internal/model"
	"github.com/reconcile/internal/pattern"
)

func makeExceptions(source, reason string, count int) []model.Exception {
	out := make([]model.Exception, count)
	for i := range out {
		out[i] = model.Exception{
			Record:     model.Record{ID: "R", Source: source},
			ReasonCode: reason,
		}
	}
	return out
}

func TestDetect_BelowThreshold(t *testing.T) {
	excs := makeExceptions("gateway", "DATE_DRIFT", 2)
	alerts := pattern.Detect(excs, 3)
	if len(alerts) != 0 {
		t.Errorf("expected no alerts below threshold, got %d", len(alerts))
	}
}

func TestDetect_AtThreshold(t *testing.T) {
	excs := makeExceptions("gateway", "DATE_DRIFT", 3)
	alerts := pattern.Detect(excs, 3)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert at threshold, got %d", len(alerts))
	}
	if alerts[0].ReasonCode != "DATE_DRIFT" {
		t.Errorf("wrong reason code: %s", alerts[0].ReasonCode)
	}
	if alerts[0].Source != "gateway" {
		t.Errorf("wrong source: %s", alerts[0].Source)
	}
}

func TestDetect_MultipleGroups(t *testing.T) {
	var excs []model.Exception
	excs = append(excs, makeExceptions("gateway", "DATE_DRIFT", 4)...)
	excs = append(excs, makeExceptions("bank", "AMOUNT_MISMATCH", 2)...)
	excs = append(excs, makeExceptions("ledger", "NO_COUNTERPART", 5)...)

	alerts := pattern.Detect(excs, 3)
	if len(alerts) != 2 {
		t.Errorf("expected 2 alerts (gateway+ledger), got %d", len(alerts))
	}
}

func TestDetect_EmptyInput(t *testing.T) {
	alerts := pattern.Detect(nil, 3)
	if len(alerts) != 0 {
		t.Errorf("expected no alerts for empty input, got %d", len(alerts))
	}
}
