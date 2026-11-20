package service

import (
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

func albumOutput(m model.Album) output.Album {
	return output.Album{
		ID:        m.ID,
		TenantID:  m.TenantID,
		ProjectID: m.ProjectID,
		ParentID:  m.ParentID,
		Name:      m.Name,
		IsPrivate: m.IsPrivate,
		CreatedBy: m.CreatedBy,
		CreatedAt: m.CreatedAt,
	}
}
