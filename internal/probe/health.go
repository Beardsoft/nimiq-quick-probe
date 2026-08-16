package probe

import "fmt"

type SyncStatus struct {
	IsEstablished        bool   `json:"isEstablished"`
	SyncedValidityWindow bool   `json:"syncedValidityWindow"`
	CurrentBlock         uint64 `json:"currentBlock"`
	RemainingBlocks      uint64 `json:"remainingBlocks"`
	StateSyncProgress    uint64 `json:"stateSyncProgress"`
}

type CheckResult struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail,omitempty"`
	Error  string `json:"error,omitempty"`
}

type Report struct {
	Healthy bool          `json:"healthy"`
	Checks  []CheckResult `json:"checks"`
}

func Evaluate(cfg Config, sync *SyncStatus, syncErr error, peers int, peersErr error) Report {
	var checks []CheckResult
	healthy := true

	if cfg.CheckConsensus {
		c := CheckResult{Name: "consensus"}
		switch {
		case syncErr != nil:
			c.Error = syncErr.Error()
		case sync == nil:
			c.Error = "missing sync status"
		case !sync.IsEstablished:
			c.Detail = "consensus not established"
		default:
			c.Passed = true
			c.Detail = "established"
		}
		if !c.Passed {
			healthy = false
		}
		checks = append(checks, c)
	}

	if cfg.CheckSync {
		c := CheckResult{Name: "sync"}
		switch {
		case syncErr != nil:
			c.Error = syncErr.Error()
		case sync == nil:
			c.Error = "missing sync status"
		case sync.RemainingBlocks != 0 || sync.StateSyncProgress != 100:
			c.Detail = fmt.Sprintf("remainingBlocks=%d stateSyncProgress=%d", sync.RemainingBlocks, sync.StateSyncProgress)
		default:
			c.Passed = true
			c.Detail = fmt.Sprintf("block=%d", sync.CurrentBlock)
		}
		if !c.Passed {
			healthy = false
		}
		checks = append(checks, c)
	}

	if cfg.MinPeers > 0 {
		c := CheckResult{Name: "peers"}
		switch {
		case peersErr != nil:
			c.Error = peersErr.Error()
		case peers < cfg.MinPeers:
			c.Detail = fmt.Sprintf("%d < %d", peers, cfg.MinPeers)
		default:
			c.Passed = true
			c.Detail = fmt.Sprintf("%d >= %d", peers, cfg.MinPeers)
		}
		if !c.Passed {
			healthy = false
		}
		checks = append(checks, c)
	}

	return Report{Healthy: healthy, Checks: checks}
}
