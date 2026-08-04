package main

import (
	"math"
	"testing"
)

func TestDependencyTransferPaysLatencyPerFileWithSerialTransfers(t *testing.T) {
	dependency := Dependency{DataMB: 100, FileCount: 17}
	got := dependencyTransferSeconds(dependency, 100, 0.050)
	want := 1.0 + 17*0.050
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("transfer: got %v, want %v", got, want)
	}
}

func TestDependencyTransferTreatsMissingFileCountAsOneFile(t *testing.T) {
	dependency := Dependency{DataMB: 100}
	got := dependencyTransferSeconds(dependency, 100, 0.050)
	want := 1.050
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("transfer: got %v, want %v", got, want)
	}
}
