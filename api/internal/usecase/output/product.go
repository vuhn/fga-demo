package output

import (
	"time"
)

// Product is the usecase output copy of a product.
type Product struct {
	Name      string
	Enabled   bool
	UpdatedAt time.Time
}

// ProductLister is the result of ProductLister.List.
type ProductLister struct {
	Items []Product
}

// ProductSetter is the result of ProductSetter.Set.
type ProductSetter struct {
	Items []Product
}
