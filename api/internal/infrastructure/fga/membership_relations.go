package fga

import (
	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
)

type MembershipRelations struct{}

var _ gateway.MembershipRelationGateway = MembershipRelations{}

func roleTuple(tenantID, userID, role string) entity.Tuple {
	return entity.Tuple{User: userRef(userID), Relation: role, Object: obj(entity.ObjectTypeTenant, tenantID)}
}

// AssignRole writes the new role relation and deletes the other three, so a
// person holds exactly one direct role on the tenant.
func (MembershipRelations) AssignRole(tenantID, userID, affiliation, role string) (writes, deletes []entity.Tuple, err error) {
	if err := entity.CheckAffiliationRole(affiliation, role); err != nil {
		return nil, nil, err
	}
	for _, r := range entity.AllRoles {
		if r == role {
			writes = append(writes, roleTuple(tenantID, userID, r))
		} else {
			deletes = append(deletes, roleTuple(tenantID, userID, r))
		}
	}
	return writes, deletes, nil
}

// Removal deletes all four role relations and each direct user grant.
// Userset grants are never named here.
func (MembershipRelations) Removal(tenantID, userID string, grants []entity.Grant) []entity.Tuple {
	var deletes []entity.Tuple
	for _, r := range entity.AllRoles {
		deletes = append(deletes, roleTuple(tenantID, userID, r))
	}
	for _, g := range grants {
		if g.SubjectType != entity.SubjectUser || g.UserID == nil || *g.UserID != userID {
			continue
		}
		deletes = append(deletes, entity.Tuple{User: userRef(userID), Relation: g.Relation, Object: obj(g.ObjectType, g.ObjectID)})
	}
	return deletes
}

// ClearRoles deletes every built-in role tuple. Missing ones are ignored by the write.
func (MembershipRelations) ClearRoles(tenantID, userID string) []entity.Tuple {
	deletes := make([]entity.Tuple, 0, len(entity.AllRoles))
	for _, r := range entity.AllRoles {
		deletes = append(deletes, roleTuple(tenantID, userID, r))
	}
	return deletes
}

// Grants returns the tuple of each existing grant row.
func (MembershipRelations) Grants(grants []entity.Grant) ([]entity.Tuple, error) {
	out := make([]entity.Tuple, 0, len(grants))
	for _, g := range grants {
		t, err := grantTuple(g.ObjectType, g)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}
