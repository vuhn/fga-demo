// Package usecase declares one interface per application operation. The
// service package implements them; handlers depend only on these interfaces.
// It imports only domain packages: never Gin, pgx, or the OpenFGA SDK.
package usecase
