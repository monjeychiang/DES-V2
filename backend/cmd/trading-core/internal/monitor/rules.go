//go:build ignore

package monitor

import "trading-core/internal/risk"

// RuleEvaluator would inspect risk results and trigger alerts.
type RuleEvaluator struct{}

func (r *RuleEvaluator) Check(result risk.Result) (bool, string) {
	if !result.Allowed && result.Reason != "" {
		return true, result.Reason
	}
	return false, ""
}
