package fga

import (
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

type BlueprintRelations struct{}

var _ gateway.BlueprintRelationGateway = BlueprintRelations{}

// Created links a blueprint to its tenant, project, and owner. Versions are
// Postgres rows only.
func (BlueprintRelations) Created(b model.Blueprint) []model.Tuple {
	o := obj(model.ObjectTypeBlueprint, b.ID)
	return []model.Tuple{
		{User: obj(model.ObjectTypeTenant, b.TenantID), Relation: "tenant", Object: o},
		{User: obj(model.ObjectTypeProject, b.ProjectID), Relation: "project", Object: o},
		{User: userRef(b.CreatedBy), Relation: model.RelOwner, Object: o},
	}
}

func (BlueprintRelations) Grant(g model.Grant) (model.Tuple, error) {
	return grantTuple(model.ObjectTypeBlueprint, g)
}
