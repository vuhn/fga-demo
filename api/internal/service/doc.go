// Package service implements the usecase interfaces. Each service is one
// struct with its dependencies injected through its constructor: gateways for
// data, usecase.Authorizer for allow/deny, and helper.WriteGuard for the
// fail-closed write ordering.
package service
