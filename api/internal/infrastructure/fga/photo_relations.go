package fga

import (
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

type PhotoRelations struct{}

var _ gateway.PhotoRelationGateway = PhotoRelations{}

func (PhotoRelations) Created(p model.Photo) []model.Tuple {
	o := obj(model.ObjectTypePhoto, p.ID)
	return []model.Tuple{
		{User: obj(model.ObjectTypeTenant, p.TenantID), Relation: "tenant", Object: o},
		{User: obj(model.ObjectTypeProject, p.ProjectID), Relation: "project", Object: o},
		{User: obj(model.ObjectTypeAlbum, p.AlbumID), Relation: "album", Object: o},
		{User: userRef(p.CreatedBy), Relation: model.RelOwner, Object: o},
	}
}

func (PhotoRelations) Grant(g model.Grant) (model.Tuple, error) {
	return grantTuple(model.ObjectTypePhoto, g)
}
