package usecase

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
)

type MemberLister interface {
	List(ctx context.Context, actor, tenantID, affiliation string) ([]entity.Membership, error)
}

// AddMemberInput names the person by UserID or by Email. An unknown email
// creates the user (Name, Password, and CompanyID are then required);
// a known email reuses the one account. Affiliation is derived from the
// user's company, never sent.
type AddMemberInput struct {
	UserID    string
	Email     string
	Name      string
	Password  string
	CompanyID string
	Role      string
}

type MemberAdder interface {
	Add(ctx context.Context, actor, tenantID string, in AddMemberInput) (entity.Membership, error)
}

type MemberRoleChanger interface {
	ChangeRole(ctx context.Context, actor, tenantID, userID, role string) (entity.Membership, error)
}

type MemberRemover interface {
	Remove(ctx context.Context, actor, tenantID, userID string) error
}
