package service

import (
	"context"
	"errors"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

// roleAssignment is one membership role: a built-in role, or one custom role
// of the same affiliation. previousCustomID is the custom role the member
// holds now, or empty.
type roleAssignment struct {
	StoredRole   string
	CustomRoleID string
	Writes       []model.Tuple
	Deletes      []model.Tuple
}

func assignMemberRole(ctx context.Context, roles gateway.RoleGateway, memberRels gateway.MembershipRelationGateway, roleRels gateway.RoleRelationGateway, tenantID, userID, affiliation, role, previousCustomID string) (roleAssignment, error) {
	err := model.CheckAffiliationRole(affiliation, role)
	if errors.Is(err, model.ErrAffiliationRoleMismatch) {
		return roleAssignment{}, err
	}
	if err == nil {
		writes, deletes, err := memberRels.AssignRole(model.AssignMemberRoleParams{TenantID: tenantID, UserID: userID, Affiliation: affiliation, Role: role})
		if err != nil {
			return roleAssignment{}, err
		}
		if previousCustomID != "" {
			deletes = append(deletes, roleRels.Assignee(previousCustomID, userID))
		}
		return roleAssignment{StoredRole: role, Writes: writes, Deletes: deletes}, nil
	}
	if !model.ValidUUID(role) {
		return roleAssignment{}, model.Invalid("unknown role %q", role)
	}
	custom, err := roles.Get(ctx, tenantID, role)
	if err != nil {
		return roleAssignment{}, err
	}
	if custom.Affiliation != affiliation {
		return roleAssignment{}, model.ErrAffiliationRoleMismatch
	}
	deletes := memberRels.ClearRoles(tenantID, userID)
	if previousCustomID != "" && previousCustomID != custom.ID {
		deletes = append(deletes, roleRels.Assignee(previousCustomID, userID))
	}
	return roleAssignment{
		StoredRole:   custom.ID,
		CustomRoleID: custom.ID,
		Writes:       []model.Tuple{roleRels.Assignee(custom.ID, userID)},
		Deletes:      deletes,
	}, nil
}
