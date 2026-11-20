package fga

import (
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

type MembershipRelations struct{}

var _ gateway.MembershipRelationGateway = MembershipRelations{}

func roleTuple(tenantID, userID, role string) model.Tuple {
	return model.Tuple{User: userRef(userID), Relation: role, Object: obj(model.ObjectTypeTenant, tenantID)}
}

// AssignRole writes the new role relation and deletes the other three, so a
// person holds exactly one direct role on the tenant.
func (MembershipRelations) AssignRole(p model.AssignMemberRoleParams) (writes, deletes []model.Tuple, err error) {
	if err := model.CheckAffiliationRole(p.Affiliation, p.Role); err != nil {
		return nil, nil, err
	}
	for _, r := range model.AllRoles {
		if r == p.Role {
			writes = append(writes, roleTuple(p.TenantID, p.UserID, r))
		} else {
			deletes = append(deletes, roleTuple(p.TenantID, p.UserID, r))
		}
	}
	return writes, deletes, nil
}

// Removal deletes all four role relations and each direct user grant.
// Userset grants are never named here.
func (MembershipRelations) Removal(tenantID, userID string, grants []model.Grant) []model.Tuple {
	var deletes []model.Tuple
	for _, r := range model.AllRoles {
		deletes = append(deletes, roleTuple(tenantID, userID, r))
	}
	for _, g := range grants {
		if g.SubjectType != model.SubjectUser || g.UserID == nil || *g.UserID != userID {
			continue
		}
		deletes = append(deletes, model.Tuple{User: userRef(userID), Relation: g.Relation, Object: obj(g.ObjectType, g.ObjectID)})
	}
	return deletes
}

// ClearRoles deletes every built-in role tuple. Missing ones are ignored by the write.
func (MembershipRelations) ClearRoles(tenantID, userID string) []model.Tuple {
	deletes := make([]model.Tuple, 0, len(model.AllRoles))
	for _, r := range model.AllRoles {
		deletes = append(deletes, roleTuple(tenantID, userID, r))
	}
	return deletes
}

// Grants returns the tuple of each existing grant row.
func (MembershipRelations) Grants(grants []model.Grant) ([]model.Tuple, error) {
	out := make([]model.Tuple, 0, len(grants))
	for _, g := range grants {
		t, err := grantTuple(g.ObjectType, g)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}
