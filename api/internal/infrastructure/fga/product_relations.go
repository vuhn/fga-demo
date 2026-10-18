package fga

import (
	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
)

type ProductRelations struct{}

var _ gateway.ProductRelationGateway = ProductRelations{}

func (ProductRelations) Enabled(tenantID, product string) entity.Tuple {
	return entity.Tuple{User: wildcard, Relation: product + "_enabled", Object: obj(entity.ObjectTypeTenant, tenantID)}
}
