package fga

import (
	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
)

type AlbumRelations struct{}

var _ gateway.AlbumRelationGateway = AlbumRelations{}

// Created links the album to its tenant and owner, then either to its parent
// or (top level) to the tenant as `root`, which anchors the "every ancestor
// is public" chain. A public album also gets `public`.
func (r AlbumRelations) Created(a entity.Album) []entity.Tuple {
	o := obj(entity.ObjectTypeAlbum, a.ID)
	ts := []entity.Tuple{
		{User: obj(entity.ObjectTypeTenant, a.TenantID), Relation: "tenant", Object: o},
		{User: userRef(a.CreatedBy), Relation: entity.RelOwner, Object: o},
	}
	if a.ParentID != nil {
		ts = append(ts, entity.Tuple{User: obj(entity.ObjectTypeAlbum, *a.ParentID), Relation: "parent", Object: o})
	} else {
		ts = append(ts, entity.Tuple{User: obj(entity.ObjectTypeTenant, a.TenantID), Relation: "root", Object: o})
	}
	if !a.IsPrivate {
		ts = append(ts, r.Public(a.TenantID, a.ID))
	}
	return ts
}

func (AlbumRelations) Public(tenantID, albumID string) entity.Tuple {
	return entity.Tuple{User: obj(entity.ObjectTypeTenant, tenantID), Relation: "public", Object: obj(entity.ObjectTypeAlbum, albumID)}
}

func (AlbumRelations) Grant(g entity.Grant) (entity.Tuple, error) {
	return grantTuple(entity.ObjectTypeAlbum, g)
}
