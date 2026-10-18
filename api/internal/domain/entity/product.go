package entity

import "time"

type Product struct {
	Name      string
	Enabled   bool
	UpdatedAt time.Time
}

const (
	ProductDocuments = "documents"
	ProductPhotos    = "photos"
)

// AllProducts lists every product a tenant can enable.
var AllProducts = []string{ProductDocuments, ProductPhotos}

func ValidProduct(p string) bool { return p == ProductDocuments || p == ProductPhotos }
