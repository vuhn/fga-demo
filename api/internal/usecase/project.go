package usecase

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
)

type ProjectLister interface {
	List(ctx context.Context, actor, tenantID string) ([]entity.Visible[entity.Project], error)
}

type ProjectGetter interface {
	Get(ctx context.Context, actor, tenantID, id string) (entity.Visible[entity.Project], error)
}

type ProjectCreator interface {
	Create(ctx context.Context, actor, tenantID, name string) (entity.Project, error)
}

type ProjectRenamer interface {
	Rename(ctx context.Context, actor, tenantID, id, name string) (entity.Project, error)
}

type ProjectRoleLister interface {
	List(ctx context.Context, actor, tenantID, projectID string) ([]entity.ProjectRole, error)
}

type ProjectRoleCreator interface {
	Create(ctx context.Context, actor, tenantID, projectID, name, affiliation string) (entity.ProjectRole, error)
}

type ProjectRolePermissionSetter interface {
	SetPermissions(ctx context.Context, actor, tenantID, projectID, roleID string, permissions []string) (entity.ProjectRole, error)
}

type ProjectRoleDeleter interface {
	Delete(ctx context.Context, actor, tenantID, projectID, roleID string) error
}

type ProjectMemberLister interface {
	List(ctx context.Context, actor, tenantID, projectID string) ([]entity.ProjectMember, error)
}

// ProjectMemberCandidateLister lists the tenant members who are not yet in
// the project. It is the narrow endpoint for project member managers, who
// need not be tenant member managers.
type ProjectMemberCandidateLister interface {
	ListCandidates(ctx context.Context, actor, tenantID, projectID string) ([]entity.Membership, error)
}

type AddProjectMemberInput struct {
	UserID string
	Role   string
}

type ProjectMemberAdder interface {
	Add(ctx context.Context, actor, tenantID, projectID string, in AddProjectMemberInput) (entity.ProjectMember, error)
}

type ProjectMemberRoleChanger interface {
	ChangeRole(ctx context.Context, actor, tenantID, projectID, userID, role string) (entity.ProjectMember, error)
}

type ProjectMemberRemover interface {
	Remove(ctx context.Context, actor, tenantID, projectID, userID string) error
}
