package fga

import (
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

type DocumentRelations struct{}

var _ gateway.DocumentRelationGateway = DocumentRelations{}

func (DocumentRelations) Created(d model.Document) []model.Tuple {
	o := obj(model.ObjectTypeDocument, d.ID)
	return []model.Tuple{
		{User: obj(model.ObjectTypeTenant, d.TenantID), Relation: "tenant", Object: o},
		{User: obj(model.ObjectTypeProject, d.ProjectID), Relation: "project", Object: o},
		{User: obj(model.ObjectTypeFolder, d.FolderID), Relation: "folder", Object: o},
		{User: userRef(d.CreatedBy), Relation: model.RelOwner, Object: o},
	}
}

func (DocumentRelations) Grant(g model.Grant) (model.Tuple, error) {
	return grantTuple(model.ObjectTypeDocument, g)
}
