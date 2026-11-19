package usecase

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

type ProjectLister interface {
	List(ctx context.Context, actor, tenantID string) ([]model.Visible[model.Project], error)
}

type ProjectGetter interface {
	Get(ctx context.Context, actor, tenantID, id string) (model.Visible[model.Project], error)
}

type ProjectCreator interface {
	Create(ctx context.Context, actor, tenantID, name string) (model.Project, error)
}

type ProjectRenamer interface {
	Rename(ctx context.Context, actor, tenantID, id, name string) (model.Project, error)
}

type ProjectRoleLister interface {
	List(ctx context.Context, actor, tenantID, projectID string) ([]model.ProjectRole, error)
}

type ProjectRoleCreator interface {
	Create(ctx context.Context, actor, tenantID, projectID, name, affiliation string) (model.ProjectRole, error)
}

type ProjectRolePermissionSetter interface {
	SetPermissions(ctx context.Context, actor, tenantID, projectID, roleID string, permissions []string) (model.ProjectRole, error)
}

type ProjectRoleDeleter interface {
	Delete(ctx context.Context, actor, tenantID, projectID, roleID string) error
}

type ProjectMemberLister interface {
	List(ctx context.Context, actor, tenantID, projectID string) ([]model.ProjectMember, error)
}

// ProjectMemberCandidateLister lists the tenant members who are not yet in
// the project. It is the narrow endpoint for project member managers, who
// need not be tenant member managers.
type ProjectMemberCandidateLister interface {
	ListCandidates(ctx context.Context, actor, tenantID, projectID string) ([]model.Membership, error)
}

type AddProjectMemberInput struct {
	UserID string
	Role   string
}

type ProjectMemberAdder interface {
	Add(ctx context.Context, actor, tenantID, projectID string, in AddProjectMemberInput) (model.ProjectMember, error)
}

type ProjectMemberRoleChanger interface {
	ChangeRole(ctx context.Context, actor, tenantID, projectID, userID, role string) (model.ProjectMember, error)
}

type ProjectMemberRemover interface {
	Remove(ctx context.Context, actor, tenantID, projectID, userID string) error
}
