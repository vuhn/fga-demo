package output

import (
	"encoding/json"
	"time"
)

// EstimateGetter is the result of EstimateGetter.Get.
type EstimateGetter struct {
	BlueprintEstimate
}

// EstimateLister is the result of EstimateLister.List.
type EstimateLister struct {
	Items []BlueprintEstimate
}

// EstimateMessage is the usecase output copy of a estimateMessage.
type EstimateMessage struct {
	BlueprintID string
	Seq         int
	Role        string
	Content     string
	CreatedAt   time.Time
}

// EstimateSession is the usecase output copy of a estimateSession.
type EstimateSession struct {
	BlueprintID      string
	BlueprintVersion int
	Epoch            int
	State            json.RawMessage
	Seq              int
	CreatedBy        string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	// Status is the agent's status kept in State ("interviewing" when it has none yet).
	Status string
}

// EstimateSessionCreator is the result of EstimateSessionCreator.Create.
type EstimateSessionCreator struct {
	EstimateSessionDetail
}

// EstimateSessionDetail is a blueprint's estimate session with its messages.
type EstimateSessionDetail struct {
	Session  EstimateSession
	Messages []EstimateMessage
}

// EstimateSessionGetter is the result of EstimateSessionGetter.Get.
type EstimateSessionGetter struct {
	EstimateSessionDetail
}

// EstimateSessionMessenger is the result of EstimateSessionMessenger.Send.
type EstimateSessionMessenger struct {
	EstimateSessionDetail
}
