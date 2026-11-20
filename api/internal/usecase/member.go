package usecase

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type MemberLister interface {
	List(ctx context.Context, in input.MemberLister) (output.MemberLister, error)
}

type MemberAdder interface {
	Add(ctx context.Context, in input.MemberAdder) (output.MemberAdder, error)
}

type MemberRoleChanger interface {
	ChangeRole(ctx context.Context, in input.MemberRoleChanger) (output.MemberRoleChanger, error)
}

type MemberRemover interface {
	Remove(ctx context.Context, in input.MemberRemover) error
}
