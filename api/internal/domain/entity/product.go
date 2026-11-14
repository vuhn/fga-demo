package entity

import "time"

type Product struct {
	Name      string
	Enabled   bool
	UpdatedAt time.Time
}

const (
	ProductDocuments  = "documents"
	ProductPhotos     = "photos"
	ProductBlueprints = "blueprints"
)

// AllProducts lists every product a tenant can enable.
var AllProducts = []string{ProductDocuments, ProductPhotos, ProductBlueprints}

func ValidProduct(p string) bool {
	for _, x := range AllProducts {
		if p == x {
			return true
		}
	}
	return false
}
