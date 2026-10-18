package fga

import (
	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
)

type FolderRelations struct{}

var _ gateway.FolderRelationGateway = FolderRelations{}

// Created links the folder to its tenant and owner, then either to its parent
// or (top level) to the tenant as `root`, which anchors the "every ancestor
// is public" chain. A public folder also gets `public`.
func (r FolderRelations) Created(f entity.Folder) []entity.Tuple {
	o := obj(entity.ObjectTypeFolder, f.ID)
	ts := []entity.Tuple{
		{User: obj(entity.ObjectTypeTenant, f.TenantID), Relation: "tenant", Object: o},
		{User: userRef(f.CreatedBy), Relation: entity.RelOwner, Object: o},
	}
	if f.ParentID != nil {
		ts = append(ts, entity.Tuple{User: obj(entity.ObjectTypeFolder, *f.ParentID), Relation: "parent", Object: o})
	} else {
		ts = append(ts, entity.Tuple{User: obj(entity.ObjectTypeTenant, f.TenantID), Relation: "root", Object: o})
	}
	if !f.IsPrivate {
		ts = append(ts, r.Public(f.TenantID, f.ID))
	}
	return ts
}

func (FolderRelations) Public(tenantID, folderID string) entity.Tuple {
	return entity.Tuple{User: obj(entity.ObjectTypeTenant, tenantID), Relation: "public", Object: obj(entity.ObjectTypeFolder, folderID)}
}

func (FolderRelations) Grant(g entity.Grant) (entity.Tuple, error) {
	return grantTuple(entity.ObjectTypeFolder, g)
}
