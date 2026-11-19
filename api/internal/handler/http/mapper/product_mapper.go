package mapper

import (
	"time"

	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

type ProductJSON struct {
	Product   string    `json:"product"`
	Enabled   bool      `json:"enabled"`
	UpdatedAt time.Time `json:"updated_at"`
}

func ProductsView(ps []model.Product) []ProductJSON {
	out := make([]ProductJSON, 0, len(ps))
	for _, p := range ps {
		out = append(out, ProductJSON{p.Name, p.Enabled, p.UpdatedAt})
	}
	return out
}
