package mapper

import (
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
	"time"
)

type ProductJSON struct {
	Product   string    `json:"product"`
	Enabled   bool      `json:"enabled"`
	UpdatedAt time.Time `json:"updated_at"`
}

func ProductsView(ps []output.Product) []ProductJSON {
	out := make([]ProductJSON, 0, len(ps))
	for _, p := range ps {
		out = append(out, ProductJSON{p.Name, p.Enabled, p.UpdatedAt})
	}
	return out
}
