package fga

import (
	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
)

type DocumentRelations struct{}

var _ gateway.DocumentRelationGateway = DocumentRelations{}

func (DocumentRelations) Created(d entity.Document) []entity.Tuple {
	o := obj(entity.ObjectTypeDocument, d.ID)
	return []entity.Tuple{
		{User: obj(entity.ObjectTypeTenant, d.TenantID), Relation: "tenant", Object: o},
		{User: obj(entity.ObjectTypeFolder, d.FolderID), Relation: "folder", Object: o},
		{User: userRef(d.CreatedBy), Relation: entity.RelOwner, Object: o},
	}
}

func (DocumentRelations) Grant(g entity.Grant) (entity.Tuple, error) {
	return grantTuple(entity.ObjectTypeDocument, g)
}
