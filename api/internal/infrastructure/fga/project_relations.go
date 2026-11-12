package fga

import (
	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
)

// ProjectRelations builds the project tuples.
type ProjectRelations struct{}

var _ gateway.ProjectRelationGateway = ProjectRelations{}

// Created links the project to its tenant.
func (ProjectRelations) Created(p entity.Project) []entity.Tuple {
	return []entity.Tuple{
		{User: obj(entity.ObjectTypeTenant, p.TenantID), Relation: "tenant", Object: obj(entity.ObjectTypeProject, p.ID)},
	}
}

// ProjectRoleRelations builds tuples for project-scoped custom roles. The
// permission catalog is the tenant role catalog; grants land on the project.
type ProjectRoleRelations struct{}

var _ gateway.ProjectRoleRelationGateway = ProjectRoleRelations{}

func projectRoleObj(id string) string { return obj(entity.ObjectTypeProjectRole, id) }

// Created anchors the role on its project and records its affiliation.
func (ProjectRoleRelations) Created(projectID, roleID, affiliation string) ([]entity.Tuple, error) {
	link, err := roleLink(affiliation)
	if err != nil {
		return nil, err
	}
	return []entity.Tuple{
		{User: obj(entity.ObjectTypeProject, projectID), Relation: "project", Object: projectRoleObj(roleID)},
		{User: projectRoleObj(roleID), Relation: link, Object: obj(entity.ObjectTypeProject, projectID)},
	}, nil
}

// Permissions writes one grant_* userset per assigned permission.
func (ProjectRoleRelations) Permissions(projectID, roleID string, permissions []string) ([]entity.Tuple, error) {
	out := make([]entity.Tuple, 0, len(permissions))
	for _, p := range permissions {
		rel, ok := entity.GrantRelation(p)
		if !ok {
			return nil, entity.Invalid("unknown permission %q", p)
		}
		out = append(out, entity.Tuple{
			User:     projectRoleObj(roleID) + "#assignee",
			Relation: rel,
			Object:   obj(entity.ObjectTypeProject, projectID),
		})
	}
	return out, nil
}

// Assignee names one user on the role.
func (ProjectRoleRelations) Assignee(roleID, userID string) entity.Tuple {
	return entity.Tuple{User: userRef(userID), Relation: "assignee", Object: projectRoleObj(roleID)}
}

// Removal deletes the anchors and each grant.
func (r ProjectRoleRelations) Removal(projectID, roleID, affiliation string, permissions []string) ([]entity.Tuple, error) {
	out, err := r.Created(projectID, roleID, affiliation)
	if err != nil {
		return nil, err
	}
	grants, err := r.Permissions(projectID, roleID, permissions)
	if err != nil {
		return nil, err
	}
	return append(out, grants...), nil
}

// ProjectMemberRelations builds the built-in project role tuples of a member.
type ProjectMemberRelations struct{}

var _ gateway.ProjectMemberRelationGateway = ProjectMemberRelations{}

func projectRoleTuple(projectID, userID, role string) entity.Tuple {
	return entity.Tuple{User: userRef(userID), Relation: role, Object: obj(entity.ObjectTypeProject, projectID)}
}

// AssignRole writes the new role relation and deletes the other three.
func (ProjectMemberRelations) AssignRole(projectID, userID, affiliation, role string) (writes, deletes []entity.Tuple, err error) {
	if err := entity.CheckAffiliationRole(affiliation, role); err != nil {
		return nil, nil, err
	}
	for _, r := range entity.AllRoles {
		if r == role {
			writes = append(writes, projectRoleTuple(projectID, userID, r))
		} else {
			deletes = append(deletes, projectRoleTuple(projectID, userID, r))
		}
	}
	return writes, deletes, nil
}

// ClearRoles deletes every built-in role tuple. Missing ones are ignored by the write.
func (ProjectMemberRelations) ClearRoles(projectID, userID string) []entity.Tuple {
	deletes := make([]entity.Tuple, 0, len(entity.AllRoles))
	for _, r := range entity.AllRoles {
		deletes = append(deletes, projectRoleTuple(projectID, userID, r))
	}
	return deletes
}
