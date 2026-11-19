package http

import (
	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

type grantJSON struct {
	ID       string  `json:"id"`
	Relation string  `json:"relation"`
	Subject  string  `json:"subject"`
	UserID   *string `json:"user_id"`
	Name     string  `json:"name,omitempty"`
}

func grantView(g model.Grant) grantJSON {
	return grantJSON{g.ID, g.Relation, g.SubjectType, g.UserID, g.UserName}
}

// shareBody is the share and revoke request body.
type shareBody struct {
	Relation string `json:"relation"`
	Subject  string `json:"subject"`
	UserID   string `json:"user_id"`
}

func (b shareBody) input() model.ShareInput {
	return model.ShareInput{Relation: b.Relation, Subject: b.Subject, UserID: b.UserID}
}
