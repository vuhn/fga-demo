package service

import (
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

func taskOutput(m model.Task) output.Task {
	return output.Task{
		ID:           m.ID,
		ProjectID:    m.ProjectID,
		Title:        m.Title,
		Description:  m.Description,
		AssigneeID:   m.AssigneeID,
		AssigneeName: m.AssigneeName,
		PlannedStart: m.PlannedStart,
		PlannedEnd:   m.PlannedEnd,
		ActualStart:  m.ActualStart,
		ActualEnd:    m.ActualEnd,
		Status:       m.Status,
		Weight:       m.Weight,
		CreatedBy:    m.CreatedBy,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
		CommentCount: m.CommentCount,
	}
}
