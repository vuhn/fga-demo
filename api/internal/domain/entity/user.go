package entity

import "time"

type User struct {
	ID            string
	Email         string
	Name          string
	PasswordHash  string
	IsSystemAdmin bool
	CompanyID     string
	CompanyName   string
	CreatedAt     time.Time
}
