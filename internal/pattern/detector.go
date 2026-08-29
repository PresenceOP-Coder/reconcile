package pattern

import (
	"fmt"

	"github.com/reconcile/internal/model"
)

// SystemicAlert is raised when a pattern of exceptions from the same source
// and reason code exceeds a threshold — indicating a systemic issue rather
// than isolated noise.
type SystemicAlert struct {
	ReasonCode string
	Source     string
	Count      int
	Threshold  int
	Message    string
}

// Detect scans the exception list and returns alerts for any (source, reason) group
// that hits or exceeds the given threshold.
func Detect(exceptions []model.Exception, threshold int) []SystemicAlert {
	type key struct {
		reason string
		source string
	}

	counts := make(map[key]int)
	for _, exc := range exceptions {
		counts[key{exc.ReasonCode, exc.Record.Source}]++
	}

	var alerts []SystemicAlert
	for k, count := range counts {
		if count >= threshold {
			alerts = append(alerts, SystemicAlert{
				ReasonCode: k.reason,
				Source:     k.source,
				Count:      count,
				Threshold:  threshold,
				Message:    buildMessage(k.reason, k.source, count),
			})
		}
	}

	// Sort deterministically: by reason then source
	for i := 1; i < len(alerts); i++ {
		for j := i; j > 0; j-- {
			a, b := alerts[j-1], alerts[j]
			if a.ReasonCode > b.ReasonCode || (a.ReasonCode == b.ReasonCode && a.Source > b.Source) {
				alerts[j-1], alerts[j] = alerts[j], alerts[j-1]
			}
		}
	}

	return alerts
}

func buildMessage(reason, source string, count int) string {
	switch reason {
	case "DATE_DRIFT":
		return fmt.Sprintf("Source '%s' has %d DATE_DRIFT exceptions — settlement delays appear systemic, not isolated. Check if %s has a recurring T+N lag in their batch processing.", source, count, source)
	case "AMOUNT_MISMATCH":
		return fmt.Sprintf("Source '%s' has %d AMOUNT_MISMATCH exceptions — repeated amount discrepancies suggest a fee or FX rounding issue on the %s side.", source, count, source)
	case "DUPLICATE_REF":
		return fmt.Sprintf("Source '%s' has %d DUPLICATE_REF exceptions — multiple records sharing the same ref ID indicate a double-processing bug in %s's export pipeline.", source, count, source)
	case "NO_COUNTERPART":
		return fmt.Sprintf("Source '%s' has %d NO_COUNTERPART exceptions — missing counterparts suggest %s records are not being synced to all downstream systems.", source, count, source)
	default:
		return fmt.Sprintf("Source '%s' has %d '%s' exceptions — this pattern exceeds the systemic alert threshold and warrants investigation.", source, count, reason)
	}
}
