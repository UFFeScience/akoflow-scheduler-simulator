package main

import (
	"math"
	"testing"
)

func TestApplyNetworkLatencyOverridePreservesLocalZero(t *testing.T) {
	generated := GeneratedSimulation{Matrices: Matrices{TransferDelay: map[string]map[string]float64{
		"a": {"a": 0, "b": 0.001},
		"b": {"a": 0.050, "b": 0},
	}}}
	applyNetworkLatencyOverride(&generated, 100)
	if generated.Matrices.TransferDelay["a"]["a"] != 0 {
		t.Fatal("local communication must remain zero")
	}
	if got := generated.Matrices.TransferDelay["a"]["b"]; math.Abs(got-0.1) > 1e-9 {
		t.Fatalf("override: got %v, want 0.1", got)
	}
}

func TestApplyNetworkBandwidthOverrideConvertsMbpsToMBps(t *testing.T) {
	generated := GeneratedSimulation{Matrices: Matrices{BandwidthBW: map[string]map[string]float64{
		"a": {"a": 10000, "b": 25000},
		"b": {"a": 125, "b": 10000},
	}}}
	applyNetworkBandwidthOverride(&generated, 100)
	if got := generated.Matrices.BandwidthBW["a"]["b"]; got != 12.5 {
		t.Fatalf("override: got %v MB/s, want 12.5", got)
	}
	if got := generated.Matrices.BandwidthBW["a"]["a"]; got != 10000 {
		t.Fatalf("local bandwidth changed: got %v", got)
	}
}
