package fga

import (
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

type ProductRelations struct{}

var _ gateway.ProductRelationGateway = ProductRelations{}

func (ProductRelations) Enabled(tenantID, product string) model.Tuple {
	return model.Tuple{User: wildcard, Relation: product + "_enabled", Object: obj(model.ObjectTypeTenant, tenantID)}
}
