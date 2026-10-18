package entity

import "time"

// PlatformID is the id of the single platform object.
const PlatformID = "main"

type Tenant struct {
	ID        string
	Slug      string
	Name      string
	CreatedAt time.Time
	Products  []Product
}
