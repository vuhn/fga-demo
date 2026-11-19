package fga

import (
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

type FolderRelations struct{}

var _ gateway.FolderRelationGateway = FolderRelations{}

// Created links the folder to its tenant and owner, then either to its parent
// or (top level) to the tenant as `root`, which anchors the "every ancestor
// is public" chain. A public folder also gets `public`.
func (r FolderRelations) Created(f model.Folder) []model.Tuple {
	o := obj(model.ObjectTypeFolder, f.ID)
	ts := []model.Tuple{
		{User: obj(model.ObjectTypeTenant, f.TenantID), Relation: "tenant", Object: o},
		{User: obj(model.ObjectTypeProject, f.ProjectID), Relation: "project", Object: o},
		{User: userRef(f.CreatedBy), Relation: model.RelOwner, Object: o},
	}
	if f.ParentID != nil {
		ts = append(ts, model.Tuple{User: obj(model.ObjectTypeFolder, *f.ParentID), Relation: "parent", Object: o})
	} else {
		ts = append(ts, model.Tuple{User: obj(model.ObjectTypeTenant, f.TenantID), Relation: "root", Object: o})
	}
	if !f.IsPrivate {
		ts = append(ts, r.Public(f.TenantID, f.ID))
	}
	return ts
}

func (FolderRelations) Public(tenantID, folderID string) model.Tuple {
	return model.Tuple{User: obj(model.ObjectTypeTenant, tenantID), Relation: "public", Object: obj(model.ObjectTypeFolder, folderID)}
}

func (FolderRelations) Grant(g model.Grant) (model.Tuple, error) {
	return grantTuple(model.ObjectTypeFolder, g)
}
