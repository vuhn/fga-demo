package input

import "time"

// TaskCreator is the input of TaskCreator.Create. The handler layer has already validated
// the fields (lengths, status, weight, dates, ranges); optional fields are nil when absent.
type TaskCreator struct {
	Actor        string
	TenantID     string
	ProjectID    string
	Title        string
	Description  *string
	AssigneeID   *string
	PlannedStart *time.Time
	PlannedEnd   *time.Time
	ActualStart  *time.Time
	ActualEnd    *time.Time
	Status       *string
	Weight       *float64
}
