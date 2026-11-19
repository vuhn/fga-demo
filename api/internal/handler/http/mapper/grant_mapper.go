package mapper

import (
	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

type GrantJSON struct {
	ID       string  `json:"id"`
	Relation string  `json:"relation"`
	Subject  string  `json:"subject"`
	UserID   *string `json:"user_id"`
	Name     string  `json:"name,omitempty"`
}

func GrantView(g model.Grant) GrantJSON {
	return GrantJSON{g.ID, g.Relation, g.SubjectType, g.UserID, g.UserName}
}

// ShareBody is the share and revoke request body.
type ShareBody struct {
	Relation string `json:"relation"`
	Subject  string `json:"subject"`
	UserID   string `json:"user_id"`
}

func (b ShareBody) Input() model.ShareInput {
	return model.ShareInput{Relation: b.Relation, Subject: b.Subject, UserID: b.UserID}
}
