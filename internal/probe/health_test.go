package probe

import (
	"testing"
	"time"
)

func TestEvaluateHealthy(t *testing.T) {
	now := time.UnixMilli(1_700_000_015_000)
	cfg := Config{CheckConsensus: true, CheckSync: true, MaxBlockAge: 15 * time.Second, MinPeers: 1}
	consensus := true
	head := &Head{Number: 8908524, Timestamp: 1_700_000_014_000}
	report := Evaluate(cfg, now, &consensus, nil, head, nil, 4, nil)
	if !report.Healthy {
		t.Fatalf("expected healthy, got %+v", report)
	}
	if len(report.Checks) != 3 {
		t.Fatalf("checks = %d", len(report.Checks))
	}
}

func TestEvaluateConsensusFail(t *testing.T) {
	now := time.UnixMilli(1_700_000_000_000)
	cfg := Config{CheckConsensus: true}
	consensus := false
	report := Evaluate(cfg, now, &consensus, nil, nil, nil, 0, nil)
	if report.Healthy {
		t.Fatal("expected unhealthy")
	}
}

func TestEvaluateStaleHeadFail(t *testing.T) {
	now := time.UnixMilli(1_700_000_030_000)
	cfg := Config{CheckSync: true, MaxBlockAge: 15 * time.Second}
	head := &Head{Number: 10, Timestamp: 1_700_000_000_000} // 30s old
	report := Evaluate(cfg, now, nil, nil, head, nil, 0, nil)
	if report.Healthy {
		t.Fatal("expected unhealthy")
	}
	if report.Checks[0].Name != "head" || report.Checks[0].Passed {
		t.Fatalf("%+v", report.Checks)
	}
}

func TestEvaluateFreshHeadPass(t *testing.T) {
	now := time.UnixMilli(1_700_000_005_000)
	cfg := Config{CheckSync: true, MaxBlockAge: 15 * time.Second}
	head := &Head{Number: 10, Timestamp: 1_700_000_004_000} // 1s old
	report := Evaluate(cfg, now, nil, nil, head, nil, 0, nil)
	if !report.Healthy {
		t.Fatalf("expected healthy, got %+v", report)
	}
}

func TestEvaluatePeersFail(t *testing.T) {
	now := time.UnixMilli(1_700_000_000_000)
	cfg := Config{MinPeers: 3}
	report := Evaluate(cfg, now, nil, nil, nil, nil, 1, nil)
	if report.Healthy {
		t.Fatal("expected unhealthy")
	}
}

func TestEvaluateDisabledChecksSkipped(t *testing.T) {
	now := time.UnixMilli(1_700_000_000_000)
	cfg := Config{CheckConsensus: false, CheckSync: false, MaxBlockAge: 0, MinPeers: 0}
	report := Evaluate(cfg, now, nil, nil, nil, nil, 0, nil)
	if !report.Healthy {
		t.Fatal("expected healthy when no checks enabled")
	}
	if len(report.Checks) != 0 {
		t.Fatalf("expected no checks, got %+v", report.Checks)
	}
}

func TestEvaluateRPCErrorFailsEnabledChecks(t *testing.T) {
	now := time.UnixMilli(1_700_000_000_000)
	cfg := Config{CheckConsensus: true, CheckSync: true, MaxBlockAge: 15 * time.Second, MinPeers: 1}
	report := Evaluate(cfg, now, nil, errTest, nil, errTest, 0, errTest)
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
