package model

import "time"

// Task statuses (closed set; the database CHECK carries the same list).
const (
	TaskNotStarted = "not_started"
	TaskInProgress = "in_progress"
	TaskCompleted  = "completed"
	TaskBlocked    = "blocked"
)

// ValidTaskStatus reports whether s is one of the four statuses.
func ValidTaskStatus(s string) bool {
	switch s {
	case TaskNotStarted, TaskInProgress, TaskCompleted, TaskBlocked:
		return true
	}
	return false
}

// Task is one work item of a project. It has no OpenFGA object: access is decided on the project.
type Task struct {
	ID          string
	TenantID    string
	ProjectID   string
	Title       string
	Description string
	AssigneeID  *string
	// AssigneeName is the assignee's display name, read by the join; empty when unassigned.
	AssigneeName *string
	// Dates are calendar dates (no time of day, no time zone).
	PlannedStart *time.Time
	PlannedEnd   *time.Time
	ActualStart  *time.Time
	ActualEnd    *time.Time
	Status       string
	// Weight is positive with at most two decimals (numeric(8,2) in Postgres).
	Weight       float64
	CreatedBy    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	CommentCount int
}
