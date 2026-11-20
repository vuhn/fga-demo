package service

import (
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

func folderOutput(m model.Folder) output.Folder {
	return output.Folder{
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
