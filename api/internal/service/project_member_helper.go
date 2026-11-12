package service

import (
	"context"
	"errors"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
)

// projectRoleAssignment is one project role: a built-in role, or one project
// role of the member's affiliation.
type projectRoleAssignment struct {
	StoredRole    string
	ProjectRoleID string
	Writes        []entity.Tuple
	Deletes       []entity.Tuple
}

// assignProjectRole mirrors assignMemberRole for the project level. The
// affiliation is the tenant member's derived affiliation, so an internal role
// never reaches an external member and the reverse.
func assignProjectRole(ctx context.Context, roles gateway.ProjectRoleGateway, memberRels gateway.ProjectMemberRelationGateway, roleRels gateway.ProjectRoleRelationGateway, projectID, userID, affiliation, role, previousRoleID string) (projectRoleAssignment, error) {
	err := entity.CheckAffiliationRole(affiliation, role)
	if errors.Is(err, entity.ErrAffiliationRoleMismatch) {
		return projectRoleAssignment{}, err
	}
	if err == nil {
		writes, deletes, err := memberRels.AssignRole(projectID, userID, affiliation, role)
		if err != nil {
			return projectRoleAssignment{}, err
		}
		if previousRoleID != "" {
			deletes = append(deletes, roleRels.Assignee(previousRoleID, userID))
		}
		return projectRoleAssignment{StoredRole: role, Writes: writes, Deletes: deletes}, nil
	}
	if !entity.ValidUUID(role) {
		return projectRoleAssignment{}, entity.Invalid("unknown role %q", role)
	}
	custom, err := roles.Get(ctx, projectID, role)
	if err != nil {
		return projectRoleAssignment{}, err
	}
	if custom.Affiliation != affiliation {
		return projectRoleAssignment{}, entity.ErrAffiliationRoleMismatch
	}
	deletes := memberRels.ClearRoles(projectID, userID)
	if previousRoleID != "" && previousRoleID != custom.ID {
		deletes = append(deletes, roleRels.Assignee(previousRoleID, userID))
	}
	return projectRoleAssignment{
		StoredRole:    custom.ID,
		ProjectRoleID: custom.ID,
		Writes:        []entity.Tuple{roleRels.Assignee(custom.ID, userID)},
		Deletes:       deletes,
	}, nil
}
