package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
)

// roleAssignment is one membership role: a built-in role, or one custom role
// of the same affiliation. previousCustomID is the custom role the member
// holds now, or empty.
type roleAssignment struct {
	StoredRole   string
	CustomRoleID string
	Writes       []entity.Tuple
	Deletes      []entity.Tuple
}

func assignMemberRole(ctx context.Context, roles gateway.RoleGateway, memberRels gateway.MembershipRelationGateway, roleRels gateway.RoleRelationGateway, tenantID, userID, affiliation, role, previousCustomID string) (roleAssignment, error) {
	if err := entity.CheckAffiliationRole(affiliation, role); err == nil {
		writes, deletes, err := memberRels.AssignRole(tenantID, userID, affiliation, role)
		if err != nil {
			return roleAssignment{}, err
		}
		if previousCustomID != "" {
			deletes = append(deletes, roleRels.Assignee(previousCustomID, userID))
		}
		return roleAssignment{StoredRole: role, Writes: writes, Deletes: deletes}, nil
	}
	if !entity.ValidUUID(role) {
		return roleAssignment{}, entity.Invalid("unknown role %q", role)
	}
	custom, err := roles.Get(ctx, tenantID, role)
	if err != nil {
		return roleAssignment{}, err
	}
	if custom.Affiliation != affiliation {
		return roleAssignment{}, entity.ErrAffiliationRoleMismatch
	}
	deletes := memberRels.ClearRoles(tenantID, userID)
	if previousCustomID != "" && previousCustomID != custom.ID {
		deletes = append(deletes, roleRels.Assignee(previousCustomID, userID))
	}
	return roleAssignment{
		StoredRole:   custom.ID,
		CustomRoleID: custom.ID,
		Writes:       []entity.Tuple{roleRels.Assignee(custom.ID, userID)},
		Deletes:      deletes,
	}, nil
}
