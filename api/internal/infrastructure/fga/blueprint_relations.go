package fga

import (
	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
)

type BlueprintRelations struct{}

var _ gateway.BlueprintRelationGateway = BlueprintRelations{}

// Created links a blueprint to its tenant, project, and owner. Versions are
// Postgres rows only.
func (BlueprintRelations) Created(b entity.Blueprint) []entity.Tuple {
	o := obj(entity.ObjectTypeBlueprint, b.ID)
	return []entity.Tuple{
		{User: obj(entity.ObjectTypeTenant, b.TenantID), Relation: "tenant", Object: o},
		{User: obj(entity.ObjectTypeProject, b.ProjectID), Relation: "project", Object: o},
		{User: userRef(b.CreatedBy), Relation: entity.RelOwner, Object: o},
	}
}

func (BlueprintRelations) Grant(g entity.Grant) (entity.Tuple, error) {
	return grantTuple(entity.ObjectTypeBlueprint, g)
}
