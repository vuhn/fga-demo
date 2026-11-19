package http

import (
	"time"

	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

type productJSON struct {
	Product   string    `json:"product"`
	Enabled   bool      `json:"enabled"`
	UpdatedAt time.Time `json:"updated_at"`
}

func productsView(ps []model.Product) []productJSON {
	out := make([]productJSON, 0, len(ps))
	for _, p := range ps {
		out = append(out, productJSON{p.Name, p.Enabled, p.UpdatedAt})
	}
	return out
}
