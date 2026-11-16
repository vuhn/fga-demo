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

// EstimateSession is a chat with the cost-estimation agent about one blueprint version. State is the agent's
// own JSON (facts, room scope, researched rates, the latest estimate): opaque to Go. Seq counts the turns and
// guards concurrent turns the way base_version guards blueprint edits.
type EstimateSession struct {
	ID               string
	TenantID         string
	BlueprintID      string
	BlueprintVersion int
	Status           string
	State            json.RawMessage
	Seq              int
	CreatedBy        string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type EstimateMessage struct {
	SessionID string
	Seq       int
	Role      string // user | assistant
	Content   string
	CreatedAt time.Time
}

// BlueprintEstimate is one finished, priced estimate. Result holds the lines, markups, totals and every rate
// with its sources; the totals are copied out for lists.
type BlueprintEstimate struct {
	ID          string
	SessionID   string
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
