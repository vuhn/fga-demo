package output

import "time"

// Task is the usecase output copy of a task.
type Task struct {
	ID           string
	ProjectID    string
	Title        string
	Description  string
	AssigneeID   *string
	AssigneeName *string
	PlannedStart *time.Time
	PlannedEnd   *time.Time
	ActualStart  *time.Time
	ActualEnd    *time.Time
	Status       string
	Weight       float64
	CreatedBy    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	CommentCount int
}

// TaskCreator is the result of TaskCreator.Create: the task with the caller's permissions on its project.
type TaskCreator struct {
	Visible[Task]
}
