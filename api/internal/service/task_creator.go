package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type taskCreator struct {
	projects gateway.ProjectGateway
	tasks    gateway.TaskGateway
	authz    helper.Authorizer
	ids      gateway.IDGenerator
}

func NewTaskCreator(projects gateway.ProjectGateway, tasks gateway.TaskGateway, authz helper.Authorizer, ids gateway.IDGenerator) usecase.TaskCreator {
	return &taskCreator{projects: projects, tasks: tasks, authz: authz, ids: ids}
}

// Create needs can_edit_tasks on the project (a project of another tenant is 404). Field
// validation lives in the handler layer; an assignee must pass can_access on the project.
func (s *taskCreator) Create(ctx context.Context, in input.TaskCreator) (output.TaskCreator, error) {
	if _, err := loadProject(ctx, s.projects, in.TenantID, in.ProjectID); err != nil {
		return output.TaskCreator{}, err
	}
	if err := s.authz.Require(ctx, in.Actor, "can_edit_tasks", model.ObjectTypeProject, in.ProjectID); err != nil {
		return output.TaskCreator{}, err
	}
	if in.AssigneeID != nil {
		if err := s.requireAssignee(ctx, *in.AssigneeID, in.ProjectID); err != nil {
			return output.TaskCreator{}, err
		}
	}
	t := s.newTask(in)
	if err := s.tasks.Create(ctx, t); err != nil {
		return output.TaskCreator{}, err
	}
	got, err := s.tasks.Get(ctx, in.TenantID, t.ID)
	if err != nil {
		return output.TaskCreator{}, err
	}
	perms, err := s.authz.Permissions(ctx, in.Actor, model.ObjectTypeProject, []string{in.ProjectID})
	if err != nil {
		return output.TaskCreator{}, err
	}
	return output.TaskCreator{Visible: output.Visible[output.Task]{Value: taskOutput(got), Perms: perms[in.ProjectID]}}, nil
}

// requireAssignee decides with OpenFGA only (never the assignee's role or affiliation).
func (s *taskCreator) requireAssignee(ctx context.Context, assigneeID, projectID string) error {
	ok, err := s.authz.Check(ctx, assigneeID, "can_access", model.ObjectTypeProject, projectID)
	if err != nil {
		return err
	}
	if !ok {
		return model.ErrNotAMember
	}
	return nil
}

// newTask applies the defaults: status not_started, weight 1, empty description.
func (s *taskCreator) newTask(in input.TaskCreator) model.Task {
	t := model.Task{ID: s.ids.NewID(), TenantID: in.TenantID, ProjectID: in.ProjectID, Title: in.Title,
		AssigneeID: in.AssigneeID, PlannedStart: in.PlannedStart, PlannedEnd: in.PlannedEnd, ActualStart: in.ActualStart, ActualEnd: in.ActualEnd,
		Status: model.TaskNotStarted, Weight: 1, CreatedBy: in.Actor}
	if in.Description != nil {
		t.Description = *in.Description
	}
	if in.Status != nil {
		t.Status = *in.Status
	}
	if in.Weight != nil {
		t.Weight = *in.Weight
	}
	return t
}
