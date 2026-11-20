package service

import (
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

func productOutput(m model.Product) output.Product {
	return output.Product{
		Name:      m.Name,
		Enabled:   m.Enabled,
		UpdatedAt: m.UpdatedAt,
	}
}
