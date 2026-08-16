package probe

import "testing"

func TestEvaluateHealthy(t *testing.T) {
	cfg := Config{CheckConsensus: true, CheckSync: true, MinPeers: 1}
	sync := &SyncStatus{IsEstablished: true, RemainingBlocks: 0, StateSyncProgress: 100}
	report := Evaluate(cfg, sync, nil, 4, nil)
	if !report.Healthy {
		t.Fatalf("expected healthy, got %+v", report)
	}
	if len(report.Checks) != 3 {
		t.Fatalf("checks = %d", len(report.Checks))
	}
}

func TestEvaluateConsensusFail(t *testing.T) {
	cfg := Config{CheckConsensus: true}
	sync := &SyncStatus{IsEstablished: false, RemainingBlocks: 0, StateSyncProgress: 100}
	report := Evaluate(cfg, sync, nil, 0, nil)
	if report.Healthy {
		t.Fatal("expected unhealthy")
	}
}

func TestEvaluateSyncFail(t *testing.T) {
	cfg := Config{CheckSync: true}
	sync := &SyncStatus{IsEstablished: true, RemainingBlocks: 12, StateSyncProgress: 40}
	report := Evaluate(cfg, sync, nil, 0, nil)
	if report.Healthy {
		t.Fatal("expected unhealthy")
	}
}

func TestEvaluatePeersFail(t *testing.T) {
	cfg := Config{MinPeers: 3}
	report := Evaluate(cfg, nil, nil, 1, nil)
	if report.Healthy {
		t.Fatal("expected unhealthy")
	}
}

func TestEvaluateDisabledChecksSkipped(t *testing.T) {
	cfg := Config{CheckConsensus: false, CheckSync: false, MinPeers: 0}
	report := Evaluate(cfg, nil, nil, 0, nil)
	if !report.Healthy {
		t.Fatal("expected healthy when no checks enabled")
	}
	if len(report.Checks) != 0 {
		t.Fatalf("expected no checks, got %+v", report.Checks)
	}
}

func TestEvaluateRPCErrorFailsEnabledChecks(t *testing.T) {
	cfg := Config{CheckConsensus: true, CheckSync: true, MinPeers: 1}
	report := Evaluate(cfg, nil, errTest, 0, errTest)
	if report.Healthy {
		t.Fatal("expected unhealthy")
	}
	if len(report.Checks) != 3 {
		t.Fatalf("checks = %d", len(report.Checks))
	}
	for _, c := range report.Checks {
		if c.Passed {
			t.Fatalf("check %s should fail", c.Name)
		}
		if c.Error == "" {
			t.Fatalf("check %s missing error", c.Name)
		}
	}
}

var errTest = errString("rpc down")

type errString string

func (e errString) Error() string { return string(e) }
