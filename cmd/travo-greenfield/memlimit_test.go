package main

import "testing"

func TestMemoryLimitDefaultsAndRespectsGOMEMLIMIT(t *testing.T) {
	if limit, ok := memoryLimit(""); !ok || limit != 32<<20 {
		t.Fatalf("default = %d %v, want 32 MiB", limit, ok)
	}
	if _, ok := memoryLimit("48MiB"); ok {
		t.Fatal("an explicit GOMEMLIMIT is the runtime's; the daemon must not override it")
	}
}
