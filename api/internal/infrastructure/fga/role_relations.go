package fga

import (
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

// RoleRelations builds tuples for tenant-scoped custom roles.
type RoleRelations struct{}

var _ gateway.RoleRelationGateway = RoleRelations{}

func roleObj(id string) string         { return obj(model.ObjectTypeTenantRole, id) }
func assigneeSet(roleID string) string { return roleObj(roleID) + "#assignee" }

func roleLink(affiliation string) (string, error) {
	switch affiliation {
	case model.AffiliationInternal:
		return "internal_role", nil
	case model.AffiliationExternal:
		return "external_role", nil
	default:
		return "", model.Invalid("unknown affiliation %q", affiliation)
	}
}

// Created anchors the role on its tenant and records which affiliation it belongs to.
func (RoleRelations) Created(tenantID, roleID, affiliation string) ([]model.Tuple, error) {
	link, err := roleLink(affiliation)
	if err != nil {
		return nil, err
	}
	return []model.Tuple{
		{User: obj(model.ObjectTypeTenant, tenantID), Relation: "tenant", Object: roleObj(roleID)},
		{User: roleObj(roleID), Relation: link, Object: obj(model.ObjectTypeTenant, tenantID)},
	}, nil
}

// Permissions writes one grant_* userset per assigned permission. Implied
// permissions are not written; the model already includes them.
func (RoleRelations) Permissions(tenantID, roleID string, permissions []string) ([]model.Tuple, error) {
	out := make([]model.Tuple, 0, len(permissions))
	for _, p := range permissions {
		rel, ok := model.GrantRelation(p)
		if !ok {
			return nil, model.Invalid("unknown permission %q", p)
		}
		out = append(out, model.Tuple{
			User:     assigneeSet(roleID),
			Relation: rel,
			Object:   obj(model.ObjectTypeTenant, tenantID),
		})
	}
	return out, nil
}

// Assignee names one user on the role.
func (RoleRelations) Assignee(roleID, userID string) model.Tuple {
	return model.Tuple{User: userRef(userID), Relation: "assignee", Object: roleObj(roleID)}
}

// Removal deletes the anchors, each grant, and each assignee.
func (r RoleRelations) Removal(tenantID, roleID, affiliation string, permissions, userIDs []string) ([]model.Tuple, error) {
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
