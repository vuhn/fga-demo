package mapper

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

func TestTaskViewOmitsEmptyAndFormatsDates(t *testing.T) {
	d := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	m := TaskView(output.Visible[output.Task]{Value: output.Task{ID: "t", Title: "x", Status: "blocked", Weight: 1, PlannedStart: &d}, Perms: map[string]bool{"can_edit_tasks": true, "can_view_tasks": true}})
	if m["planned_start"] != "2026-01-05" {
		t.Errorf("date: %v", m["planned_start"])
	}
	for _, k := range []string{"description", "assignee_id", "assignee_name", "planned_end", "actual_start", "actual_end"} {
		if _, ok := m[k]; ok {
			t.Errorf("%s must be omitted when empty", k)
		}
	}
	p := m["permissions"].(map[string]bool)
	if len(p) != 2 || !p["can_edit_tasks"] || p["can_manage_tasks"] {
		t.Errorf("permissions: %v", p)
	}
}

func sp(s string) *string   { return &s }
func fp(f float64) *float64 { return &f }

func TestTaskCreatorInputRejectsInvalidBodies(t *testing.T) {
	bad := map[string]TaskCreateBody{
		"empty title":     {Title: "   "},
		"long title":      {Title: strings.Repeat("x", 201)},
		"long desc":       {Title: "x", Description: sp(strings.Repeat("d", 10001))},
		"unknown status":  {Title: "x", Status: sp("done")},
		"weight zero":     {Title: "x", Weight: fp(0)},
		"weight negative": {Title: "x", Weight: fp(-1)},
		"weight 3 dec":    {Title: "x", Weight: fp(1.005)},
		"weight huge":     {Title: "x", Weight: fp(1e7)},
		"bad date":        {Title: "x", PlannedStart: sp("05/01/2026")},
		"planned range":   {Title: "x", PlannedStart: sp("2026-02-01"), PlannedEnd: sp("2026-01-01")},
		"actual range":    {Title: "x", ActualStart: sp("2026-02-01"), ActualEnd: sp("2026-01-01")},
		"assignee id":     {Title: "x", AssigneeID: sp("not-a-uuid")},
	}
	for name, b := range bad {
		if _, err := TaskCreatorInput("a", "t", "p", b); !errors.Is(err, model.ErrValidation) {
			t.Errorf("%s: got %v, want validation", name, err)
		}
	}
}

func TestTaskCreatorInputConvertsValidBody(t *testing.T) {
	in, err := TaskCreatorInput("a", "t", "p", TaskCreateBody{Title: "  Pour slab  ", Status: sp("blocked"), Weight: fp(2.25),
		PlannedStart: sp("2026-01-05"), PlannedEnd: sp("2026-01-05"), AssigneeID: sp("11111111-1111-4111-8111-111111111111")})
	if err != nil {
		t.Fatal(err)
	}
	if in.Title != "Pour slab" || in.PlannedStart == nil || in.PlannedStart.Format("2006-01-02") != "2026-01-05" || *in.Weight != 2.25 || in.ProjectID != "p" {
		t.Fatalf("%+v", in)
	}
}
