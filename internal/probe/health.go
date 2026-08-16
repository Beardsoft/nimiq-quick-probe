package probe

import (
	"fmt"
	"time"
)

type Head struct {
	Number    uint64 `json:"number"`
	Timestamp int64  `json:"timestamp"`
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

func Evaluate(cfg Config, now time.Time, consensus *bool, consensusErr error, head *Head, headErr error, peers int, peersErr error) Report {
	var checks []CheckResult
	healthy := true

	if cfg.CheckConsensus {
		c := CheckResult{Name: "consensus"}
		switch {
		case consensusErr != nil:
			c.Error = consensusErr.Error()
		case consensus == nil:
			c.Error = "missing consensus status"
		case !*consensus:
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

	if cfg.CheckSync && cfg.MaxBlockAge > 0 {
		c := CheckResult{Name: "head"}
		switch {
		case headErr != nil:
			c.Error = headErr.Error()
		case head == nil:
			c.Error = "missing latest block"
		default:
			age := now.Sub(time.UnixMilli(head.Timestamp))
			if age < 0 {
				age = 0
			}
			if age > cfg.MaxBlockAge {
				c.Detail = fmt.Sprintf("block=%d age=%s > %s", head.Number, age.Round(time.Millisecond), cfg.MaxBlockAge)
			} else {
				c.Passed = true
				c.Detail = fmt.Sprintf("block=%d age=%s", head.Number, age.Round(time.Millisecond))
			}
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
