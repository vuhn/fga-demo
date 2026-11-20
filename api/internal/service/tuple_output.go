package service

import (
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

func tupleOutput(m model.Tuple) output.Tuple {
	return output.Tuple{User: m.User, Relation: m.Relation, Object: m.Object}
}
