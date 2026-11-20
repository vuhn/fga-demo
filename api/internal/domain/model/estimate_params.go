package model

// AppendEstimateTurnParams is one finished estimate turn to store: the new
// state, both messages and, when the turn produced one, the estimate.
type AppendEstimateTurnParams struct {
	Session   EstimateSession
	State     []byte
	User      EstimateMessage
	Assistant EstimateMessage
	Estimate  *BlueprintEstimate
}

// EstimateTurnParams is what the agent needs for one estimate turn: the plan
// model, the session state, the new message and the chat so far.
type EstimateTurnParams struct {
	Model     []byte
	State     []byte
	Message   string
	Messages  []EstimateMessage
	Overrides map[string]float64
}
