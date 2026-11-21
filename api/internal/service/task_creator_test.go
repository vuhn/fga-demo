package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
)

const (
	tcTenant  = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	tcProject = "a1a1a1a1-0000-4000-8000-000000000001"
	tcMember  = "11111111-1111-4111-8111-111111111111"
	tcOutside = "22222222-2222-4222-8222-222222222222"
)

type tcProjects struct {
	gateway.ProjectGateway
	tenant string
}

func (p tcProjects) Get(context.Context, string) (model.Project, error) {
	return model.Project{ID: tcProject, TenantID: p.tenant}, nil
}

type tcTasks struct {
	gateway.TaskGateway
	created []model.Task
}

func (g *tcTasks) Create(_ context.Context, t model.Task) error {
	g.created = append(g.created, t)
	return nil
}
func (g *tcTasks) Get(_ context.Context, _, id string) (model.Task, error) {
	return g.created[0], nil
}

type tcIDs struct{}

func (tcIDs) NewID() string { return "33333333-3333-4333-8333-333333333333" }

// tcAuthz allows can_edit_tasks unless deny is set; can_access only for the listed users.
type tcAuthz struct {
	helper.Authorizer
	deny   bool
	access map[string]bool
	calls  []string
}

func (a *tcAuthz) Require(_ context.Context, _, relation, _, _ string) error {
	a.calls = append(a.calls, "require:"+relation)
	if a.deny {
		return &model.CheckError{Err: model.ErrForbidden, Relation: relation}
	}
	return nil
}
func (a *tcAuthz) Check(_ context.Context, user, relation, _, _ string) (bool, error) {
	a.calls = append(a.calls, "check:"+relation+":"+user)
	return a.access[user], nil
}
func (a *tcAuthz) Permissions(_ context.Context, _, _ string, ids []string) (map[string]model.Perms, error) {
	return map[string]model.Perms{ids[0]: {"can_edit_tasks": true}}, nil
}

func newTaskCreator(tenant string, deny bool) (*taskCreator, *tcTasks, *tcAuthz) {
	tasks, authz := &tcTasks{}, &tcAuthz{deny: deny, access: map[string]bool{tcMember: true}}
	return NewTaskCreator(tcProjects{tenant: tenant}, tasks, authz, tcIDs{}).(*taskCreator), tasks, authz
}

func str(s string) *string   { return &s }
func num(f float64) *float64 { return &f }
func baseTask() input.TaskCreator {
	return input.TaskCreator{Actor: "actor", TenantID: tcTenant, ProjectID: tcProject, Title: "Pour slab"}
}

func TestTaskCreatorDefaults(t *testing.T) {
	s, tasks, _ := newTaskCreator(tcTenant, false)
	out, err := s.Create(context.Background(), baseTask())
	if err != nil {
		t.Fatal(err)
	}
	got := tasks.created[0]
	if got.Status != model.TaskNotStarted || got.Weight != 1 || got.CreatedBy != "actor" || got.Description != "" {
		t.Fatalf("defaults: %+v", got)
	}
	if !out.Perms["can_edit_tasks"] {
		t.Fatal("permissions must come back with the task")
	}
}

func TestTaskCreatorAllFields(t *testing.T) {
	s, tasks, _ := newTaskCreator(tcTenant, false)
	in := baseTask()
	in.Description, in.AssigneeID, in.Status, in.Weight = str("level 2"), str(tcMember), str(model.TaskInProgress), num(2.25)
	d := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	in.PlannedStart, in.PlannedEnd, in.ActualStart = &d, &d, &d
	if _, err := s.Create(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	got := tasks.created[0]
	if got.Title != "Pour slab" || got.Weight != 2.25 || got.Status != model.TaskInProgress || got.PlannedEnd == nil || got.AssigneeID == nil {
		t.Fatalf("stored %+v", got)
	}
}

func TestTaskCreatorDeniedWritesNothing(t *testing.T) {
	s, tasks, authz := newTaskCreator(tcTenant, true)
	_, err := s.Create(context.Background(), baseTask())
	var ce *model.CheckError
	if !errors.As(err, &ce) || !errors.Is(err, model.ErrForbidden) || ce.Relation != "can_edit_tasks" {
		t.Fatalf("got %v, want 403 with can_edit_tasks", err)
	}
	if len(tasks.created) != 0 || len(authz.calls) != 1 {
		t.Fatalf("created=%d calls=%v", len(tasks.created), authz.calls)
	}
}

func TestTaskCreatorAssigneeNotAMember(t *testing.T) {
	s, tasks, authz := newTaskCreator(tcTenant, false)
	in := baseTask()
	in.AssigneeID = str(tcOutside)
	if _, err := s.Create(context.Background(), in); !errors.Is(err, model.ErrNotAMember) {
		t.Fatalf("got %v, want not_a_member", err)
	}
	if len(tasks.created) != 0 {
		t.Fatal("a row was written")
	}
	if got := authz.calls[len(authz.calls)-1]; got != "check:can_access:"+tcOutside {
		t.Fatalf("assignee must be decided by can_access, calls=%v", authz.calls)
	}
}

func TestTaskCreatorOtherTenantProject(t *testing.T) {
	s, tasks, authz := newTaskCreator("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", false)
	if _, err := s.Create(context.Background(), baseTask()); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("got %v, want 404", err)
	}
	if len(authz.calls) != 0 || len(tasks.created) != 0 {
		t.Fatalf("no Check or write may happen: %v", authz.calls)
	}
}
