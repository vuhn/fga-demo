package mapper

import (
	"math"
	"strings"
	"time"

	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

const (
	taskDateLayout     = "2006-01-02"
	maxTaskTitle       = 200
	maxTaskDescription = 10000
	maxTaskWeight      = 999999.99
)

// TaskCreateBody is the request body of POST /projects/:id/tasks.
type TaskCreateBody struct {
	Title        string   `json:"title"`
	Description  *string  `json:"description"`
	AssigneeID   *string  `json:"assignee_id"`
	PlannedStart *string  `json:"planned_start"`
	PlannedEnd   *string  `json:"planned_end"`
	ActualStart  *string  `json:"actual_start"`
	ActualEnd    *string  `json:"actual_end"`
	Status       *string  `json:"status"`
	Weight       *float64 `json:"weight"`
}

// TaskCreatorInput validates the body (400 validation) and converts it to the usecase input.
func TaskCreatorInput(actor, tenantID, projectID string, b TaskCreateBody) (input.TaskCreator, error) {
	in := input.TaskCreator{Actor: actor, TenantID: tenantID, ProjectID: projectID, AssigneeID: b.AssigneeID, Description: b.Description, Status: b.Status, Weight: b.Weight}
	title := strings.TrimSpace(b.Title)
	if n := len([]rune(title)); n < 1 || n > maxTaskTitle {
		return in, model.Invalid("title length must be 1 to %d", maxTaskTitle)
	}
	in.Title = title
	if err := validTaskFields(b); err != nil {
		return in, err
	}
	var err error
	if in.PlannedStart, in.PlannedEnd, err = taskRange("planned", b.PlannedStart, b.PlannedEnd); err != nil {
		return in, err
	}
	in.ActualStart, in.ActualEnd, err = taskRange("actual", b.ActualStart, b.ActualEnd)
	return in, err
}

func validTaskFields(b TaskCreateBody) error {
	if b.Description != nil && len([]rune(*b.Description)) > maxTaskDescription {
		return model.Invalid("description must be at most %d characters", maxTaskDescription)
	}
	if b.Status != nil && !model.ValidTaskStatus(*b.Status) {
		return model.Invalid("status must be not_started, in_progress, completed or blocked")
	}
	if b.AssigneeID != nil {
		if err := model.RequireUUID("assignee_id", *b.AssigneeID); err != nil {
			return err
		}
	}
	if b.Weight != nil {
		w, cents := *b.Weight, *b.Weight*100
		if math.IsNaN(w) || w <= 0 || w > maxTaskWeight || math.Abs(cents-math.Round(cents)) > 1e-6 {
			return model.Invalid("weight must be greater than 0, at most %.2f, with at most 2 decimals", maxTaskWeight)
		}
	}
	return nil
}

// taskRange parses a start and an end date (YYYY-MM-DD) and rejects an end before its start.
func taskRange(field string, start, end *string) (*time.Time, *time.Time, error) {
	s, err := taskDate(field+"_start", start)
	if err != nil {
		return nil, nil, err
	}
	e, err := taskDate(field+"_end", end)
	if err != nil {
		return nil, nil, err
	}
	if s != nil && e != nil && e.Before(*s) {
		return nil, nil, model.Invalid("%s_end must not be before %s_start", field, field)
	}
	return s, e, nil
}

func taskDate(field string, s *string) (*time.Time, error) {
	if s == nil {
		return nil, nil
	}
	d, err := time.Parse(taskDateLayout, *s)
	if err != nil {
		return nil, model.Invalid("%s must be a date as YYYY-MM-DD", field)
	}
	return &d, nil
}

// TaskView is the JSON of a task: optional fields are omitted when empty, dates are
// YYYY-MM-DD, and permissions carry only the two task actions the UI needs.
func TaskView(v output.Visible[output.Task]) map[string]any {
	t := v.Value
	m := map[string]any{
		"id": t.ID, "project_id": t.ProjectID, "title": t.Title, "status": t.Status, "weight": t.Weight,
		"created_by": t.CreatedBy, "created_at": t.CreatedAt, "updated_at": t.UpdatedAt, "comment_count": t.CommentCount,
		"permissions": map[string]bool{"can_edit_tasks": v.Perms["can_edit_tasks"], "can_manage_tasks": v.Perms["can_manage_tasks"]},
	}
	if t.Description != "" {
		m["description"] = t.Description
	}
	if t.AssigneeID != nil {
		m["assignee_id"] = *t.AssigneeID
	}
	if t.AssigneeName != nil {
		m["assignee_name"] = *t.AssigneeName
	}
	for k, d := range map[string]*time.Time{"planned_start": t.PlannedStart, "planned_end": t.PlannedEnd, "actual_start": t.ActualStart, "actual_end": t.ActualEnd} {
		if d != nil {
			m[k] = d.Format(taskDateLayout)
		}
	}
	return m
}
