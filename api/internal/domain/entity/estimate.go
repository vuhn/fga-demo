package entity

import (
	"encoding/json"
	"time"
)

// Estimate session statuses (the engine's agent sets them).
const (
	EstimateInterviewing = "interviewing"
	EstimateResearching  = "researching"
	EstimateReady        = "ready"
	EstimateFailed       = "failed"
)

// MaxEstimateTurns caps a session (each turn can cost LLM and search calls); MaxEstimateMessage caps one message.
const (
	MaxEstimateTurns   = 40
	MaxEstimateMessage = 2000
)

// EstimateSession is the chat with the cost-estimation agent about a blueprint: there is at most one per
// blueprint, keyed by it (starting over replaces it). It is pinned to one blueprint version. State is the
// agent's own JSON (facts, room scope, researched rates, the latest estimate, its status): opaque to Go apart
// from the status. Seq counts the turns and Epoch the restarts; a turn is saved only while both are what it
// read, the way base_version guards blueprint edits.
type EstimateSession struct {
	BlueprintID      string
	BlueprintVersion int
	Epoch            int
	State            json.RawMessage
	Seq              int
	CreatedBy        string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// Status is the agent's status kept in State ("interviewing" for a state that has none yet).
func (s EstimateSession) Status() string {
	if st := EstimateStatusOf(s.State); st != "" {
		return st
	}
	return EstimateInterviewing
}

// EstimateStatusOf reads the status field of an engine state ("" when absent or unreadable).
func EstimateStatusOf(state json.RawMessage) string {
	var v struct {
		Status string `json:"status"`
	}
	if json.Unmarshal(state, &v) != nil {
		return ""
	}
	return v.Status
}

type EstimateMessage struct {
	BlueprintID string
	Seq         int
	Role        string // user | assistant
	Content     string
	CreatedAt   time.Time
}

// BlueprintEstimate is one finished, priced estimate of a blueprint version, append-only. Result holds the
// lines, markups, totals and every rate with its sources; the totals are copied out for lists.
type BlueprintEstimate struct {
	ID          string
	BlueprintID string
	Version     int
	TotalLow    int64
	TotalBase   int64
	TotalHigh   int64
	Result      json.RawMessage
	CreatedAt   time.Time
}

// EstimateEvent is one event of the agent's stream (progress, token, fact, question, rate, estimate), passed
// through to the browser unchanged and never stored.
type EstimateEvent struct {
	Name string
	Data json.RawMessage
}

// EstimateTurn is what the engine returns when a turn ends: the new state, the full reply text and, when the
// turn priced the plan, the estimate.
type EstimateTurn struct {
	State    json.RawMessage
	Status   string
	Reply    string
	Estimate json.RawMessage
}
