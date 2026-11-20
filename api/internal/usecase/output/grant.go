package output

import (
	"time"
)

// Grant is the usecase output copy of a grant.
type Grant struct {
	ID          string
	TenantID    string
	ObjectType  string
	ObjectID    string
	Relation    string
	SubjectType string
	UserID      *string
	UserName    string
	CreatedAt   time.Time
}
