package main

import "testing"

func anchoredResult(makespan, cost float64) SimulationResult {
	return SimulationResult{
		TimingVariables: TimingVariables{Makespan: makespan},
		CostVariables:   CostVariables{BUsed: cost},
	}
}

func anchoredOption(makespan, cost float64, feasible bool) ScheduleOption {
	return ScheduleOption{
		Makespan: makespan, BudgetUsed: cost, Feasible: feasible,
		Result: anchoredResult(makespan, cost),
	}
}

func TestPRISMTimeFallsBackToHEFTAndOnlyAcceptsStrictImprovement(t *testing.T) {
	heft := anchoredResult(100, 10)
	got := selectAnchoredPRISMResult("prism_cc_time", heft, []ScheduleOption{
		anchoredOption(110, 5, true),
		anchoredOption(100-prismImprovementEpsilon/2, 20, true),
	}, ScenarioSLA{BudgetLimit: 20, DeadlineLimit: 120})
	if got.TimingVariables.Makespan != 100 || got.CostVariables.BUsed != 10 {
		t.Fatalf("expected exact HEFT fallback, got makespan=%v cost=%v", got.TimingVariables.Makespan, got.CostVariables.BUsed)
	}
	got = selectAnchoredPRISMResult("prism_cc_time", heft, []ScheduleOption{
		anchoredOption(99, 20, false),
	}, ScenarioSLA{BudgetLimit: 20, DeadlineLimit: 120})
	if got.TimingVariables.Makespan != 99 {
		t.Fatalf("PRISM-Time did not select the globally faster complete schedule")
	}
}

func TestPRISMCostRequiresSLAAndBreaksCostTieByMakespan(t *testing.T) {
	heft := anchoredResult(100, 10)
	sla := ScenarioSLA{BudgetLimit: 12, DeadlineLimit: 120}
	got := selectAnchoredPRISMResult("prism_cc_cost", heft, []ScheduleOption{
		anchoredOption(121, 5, false),
		anchoredOption(115, 9, true),
		anchoredOption(110, 9, true),
	}, sla)
	if got.CostVariables.BUsed != 9 || got.TimingVariables.Makespan != 110 {
		t.Fatalf("unexpected cost selection: makespan=%v cost=%v", got.TimingVariables.Makespan, got.CostVariables.BUsed)
	}
}

func TestPRISMCostNeverReturnsMoreExpensiveThanHEFT(t *testing.T) {
	heft := anchoredResult(100, 10)
	got := selectAnchoredPRISMResult("prism_cc_cost", heft, []ScheduleOption{
		anchoredOption(80, 11, true),
	}, ScenarioSLA{BudgetLimit: 20, DeadlineLimit: 120})
	if got.CostVariables.BUsed != 10 || got.TimingVariables.Makespan != 100 {
		t.Fatalf("expected exact HEFT fallback")
	}
}
