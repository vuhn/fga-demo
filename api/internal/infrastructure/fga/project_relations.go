package fga

import (
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

// ProjectRelations builds the project tuples.
type ProjectRelations struct{}

var _ gateway.ProjectRelationGateway = ProjectRelations{}

// Created links the project to its tenant.
func (ProjectRelations) Created(p model.Project) []model.Tuple {
	return []model.Tuple{
		{User: obj(model.ObjectTypeTenant, p.TenantID), Relation: "tenant", Object: obj(model.ObjectTypeProject, p.ID)},
	}
}

// ProjectRoleRelations builds tuples for project-scoped custom roles. The
// permission catalog is the tenant role catalog; grants land on the project.
type ProjectRoleRelations struct{}

var _ gateway.ProjectRoleRelationGateway = ProjectRoleRelations{}

func projectRoleObj(id string) string { return obj(model.ObjectTypeProjectRole, id) }

// Created anchors the role on its project and records its affiliation.
func (ProjectRoleRelations) Created(projectID, roleID, affiliation string) ([]model.Tuple, error) {
	link, err := roleLink(affiliation)
	if err != nil {
		return nil, err
	}
	return []model.Tuple{
		{User: obj(model.ObjectTypeProject, projectID), Relation: "project", Object: projectRoleObj(roleID)},
		{User: projectRoleObj(roleID), Relation: link, Object: obj(model.ObjectTypeProject, projectID)},
	}, nil
}

// Permissions writes one grant_* userset per assigned permission.
func (ProjectRoleRelations) Permissions(projectID, roleID string, permissions []string) ([]model.Tuple, error) {
	out := make([]model.Tuple, 0, len(permissions))
	for _, p := range permissions {
		rel, ok := model.GrantRelation(p)
		if !ok {
			return nil, model.Invalid("unknown permission %q", p)
		}
		out = append(out, model.Tuple{
			User:     projectRoleObj(roleID) + "#assignee",
			Relation: rel,
			Object:   obj(model.ObjectTypeProject, projectID),
		})
	}
	return out, nil
}

// Assignee names one user on the role.
func (ProjectRoleRelations) Assignee(roleID, userID string) model.Tuple {
	return model.Tuple{User: userRef(userID), Relation: "assignee", Object: projectRoleObj(roleID)}
}

// Removal deletes the anchors and each grant.
func (r ProjectRoleRelations) Removal(projectID, roleID, affiliation string, permissions []string) ([]model.Tuple, error) {
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

func projectRoleTuple(projectID, userID, role string) model.Tuple {
	return model.Tuple{User: userRef(userID), Relation: role, Object: obj(model.ObjectTypeProject, projectID)}
}

// AssignRole writes the new role relation and deletes the other three.
func (ProjectMemberRelations) AssignRole(projectID, userID, affiliation, role string) (writes, deletes []model.Tuple, err error) {
	if err := model.CheckAffiliationRole(affiliation, role); err != nil {
		return nil, nil, err
	}
	for _, r := range model.AllRoles {
		if r == role {
			writes = append(writes, projectRoleTuple(projectID, userID, r))
		} else {
			deletes = append(deletes, projectRoleTuple(projectID, userID, r))
		}
	}
	return writes, deletes, nil
}

// ClearRoles deletes every built-in role tuple. Missing ones are ignored by the write.
func (ProjectMemberRelations) ClearRoles(projectID, userID string) []model.Tuple {
	deletes := make([]model.Tuple, 0, len(model.AllRoles))
	for _, r := range model.AllRoles {
		deletes = append(deletes, projectRoleTuple(projectID, userID, r))
	}
	return deletes
}
