package fga

import (
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

type AlbumRelations struct{}

var _ gateway.AlbumRelationGateway = AlbumRelations{}

// Created links the album to its tenant and owner, then either to its parent
// or (top level) to the tenant as `root`, which anchors the "every ancestor
// is public" chain. A public album also gets `public`.
func (r AlbumRelations) Created(a model.Album) []model.Tuple {
	o := obj(model.ObjectTypeAlbum, a.ID)
	ts := []model.Tuple{
		{User: obj(model.ObjectTypeTenant, a.TenantID), Relation: "tenant", Object: o},
		{User: obj(model.ObjectTypeProject, a.ProjectID), Relation: "project", Object: o},
		{User: userRef(a.CreatedBy), Relation: model.RelOwner, Object: o},
	}
	if a.ParentID != nil {
		ts = append(ts, model.Tuple{User: obj(model.ObjectTypeAlbum, *a.ParentID), Relation: "parent", Object: o})
	} else {
		ts = append(ts, model.Tuple{User: obj(model.ObjectTypeTenant, a.TenantID), Relation: "root", Object: o})
	}
	if !a.IsPrivate {
		ts = append(ts, r.Public(a.TenantID, a.ID))
	}
	return ts
}

func (AlbumRelations) Public(tenantID, albumID string) model.Tuple {
	return model.Tuple{User: obj(model.ObjectTypeTenant, tenantID), Relation: "public", Object: obj(model.ObjectTypeAlbum, albumID)}
}

func (AlbumRelations) Grant(g model.Grant) (model.Tuple, error) {
	return grantTuple(model.ObjectTypeAlbum, g)
}
