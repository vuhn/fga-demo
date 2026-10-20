package fga

import (
	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
)

// RoleRelations builds tuples for tenant-scoped custom roles.
type RoleRelations struct{}

var _ gateway.RoleRelationGateway = RoleRelations{}

func roleObj(id string) string         { return obj(entity.ObjectTypeRole, id) }
func assigneeSet(roleID string) string { return roleObj(roleID) + "#assignee" }

func roleLink(affiliation string) (string, error) {
	switch affiliation {
	case entity.AffiliationInternal:
		return "internal_role", nil
	case entity.AffiliationExternal:
		return "external_role", nil
	default:
		return "", entity.Invalid("unknown affiliation %q", affiliation)
	}
}

// Created anchors the role on its tenant and records which affiliation it belongs to.
func (RoleRelations) Created(tenantID, roleID, affiliation string) ([]entity.Tuple, error) {
	link, err := roleLink(affiliation)
	if err != nil {
		return nil, err
	}
	return []entity.Tuple{
		{User: obj(entity.ObjectTypeTenant, tenantID), Relation: "tenant", Object: roleObj(roleID)},
		{User: roleObj(roleID), Relation: link, Object: obj(entity.ObjectTypeTenant, tenantID)},
	}, nil
}

// Permissions writes one grant_* userset per assigned permission. Implied
// permissions are not written; the model already includes them.
func (RoleRelations) Permissions(tenantID, roleID string, permissions []string) ([]entity.Tuple, error) {
	out := make([]entity.Tuple, 0, len(permissions))
	for _, p := range permissions {
		rel, ok := entity.GrantRelation(p)
		if !ok {
			return nil, entity.Invalid("unknown permission %q", p)
		}
		out = append(out, entity.Tuple{
			User:     assigneeSet(roleID),
			Relation: rel,
			Object:   obj(entity.ObjectTypeTenant, tenantID),
		})
	}
	return out, nil
}

// Assignee names one user on the role.
func (RoleRelations) Assignee(roleID, userID string) entity.Tuple {
	return entity.Tuple{User: userRef(userID), Relation: "assignee", Object: roleObj(roleID)}
}

// Removal deletes the anchors, each grant, and each assignee.
func (r RoleRelations) Removal(tenantID, roleID, affiliation string, permissions, userIDs []string) ([]entity.Tuple, error) {
	out, err := r.Created(tenantID, roleID, affiliation)
	if err != nil {
		return nil, err
	}
	grants, err := r.Permissions(tenantID, roleID, permissions)
	if err != nil {
		return nil, err
	}
	out = append(out, grants...)
	for _, id := range userIDs {
		out = append(out, r.Assignee(roleID, id))
	}
	return out, nil
}
