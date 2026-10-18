package fga

import (
	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
)

type PhotoRelations struct{}

var _ gateway.PhotoRelationGateway = PhotoRelations{}

func (PhotoRelations) Created(p entity.Photo) []entity.Tuple {
	o := obj(entity.ObjectTypePhoto, p.ID)
	return []entity.Tuple{
		{User: obj(entity.ObjectTypeTenant, p.TenantID), Relation: "tenant", Object: o},
		{User: obj(entity.ObjectTypeAlbum, p.AlbumID), Relation: "album", Object: o},
		{User: userRef(p.CreatedBy), Relation: entity.RelOwner, Object: o},
	}
}

func (PhotoRelations) Grant(g entity.Grant) (entity.Tuple, error) {
	return grantTuple(entity.ObjectTypePhoto, g)
}
