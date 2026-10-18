package usecase

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
)

type MemberLister interface {
	List(ctx context.Context, actor, tenantID, affiliation string) ([]entity.Membership, error)
}

type AddMemberInput struct {
	UserID      string
	Affiliation string
	Role        string
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
