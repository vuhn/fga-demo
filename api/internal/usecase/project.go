package usecase

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type ProjectLister interface {
	List(ctx context.Context, in input.ProjectLister) (output.ProjectLister, error)
}

type ProjectGetter interface {
	Get(ctx context.Context, in input.ProjectGetter) (output.ProjectGetter, error)
}

type ProjectCreator interface {
	Create(ctx context.Context, in input.ProjectCreator) (output.ProjectCreator, error)
}

type ProjectRenamer interface {
	Rename(ctx context.Context, in input.ProjectRenamer) (output.ProjectRenamer, error)
}

type ProjectRoleLister interface {
	List(ctx context.Context, in input.ProjectRoleLister) (output.ProjectRoleLister, error)
}

type ProjectRoleCreator interface {
	Create(ctx context.Context, in input.ProjectRoleCreator) (output.ProjectRoleCreator, error)
}

type ProjectRolePermissionSetter interface {
	SetPermissions(ctx context.Context, in input.ProjectRolePermissionSetter) (output.ProjectRolePermissionSetter, error)
}

type ProjectRoleDeleter interface {
	Delete(ctx context.Context, in input.ProjectRoleDeleter) error
}

type ProjectMemberLister interface {
	List(ctx context.Context, in input.ProjectMemberLister) (output.ProjectMemberLister, error)
}

// ProjectMemberCandidateLister lists the tenant members who are not yet in
// the project. It is the narrow endpoint for project member managers, who
// need not be tenant member managers.
type ProjectMemberCandidateLister interface {
	ListCandidates(ctx context.Context, in input.ProjectMemberCandidateLister) (output.ProjectMemberCandidateLister, error)
}

type ProjectMemberAdder interface {
	Add(ctx context.Context, in input.ProjectMemberAdder) (output.ProjectMemberAdder, error)
}

type ProjectMemberRoleChanger interface {
	ChangeRole(ctx context.Context, in input.ProjectMemberRoleChanger) (output.ProjectMemberRoleChanger, error)
}

type ProjectMemberRemover interface {
	Remove(ctx context.Context, in input.ProjectMemberRemover) error
}
