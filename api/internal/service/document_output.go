package service

import (
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

func documentOutput(m model.Document) output.Document {
	return output.Document{
		ID:        m.ID,
		TenantID:  m.TenantID,
		ProjectID: m.ProjectID,
		FolderID:  m.FolderID,
		Name:      m.Name,
		MimeType:  m.MimeType,
		ByteSize:  m.ByteSize,
		CreatedBy: m.CreatedBy,
		CreatedAt: m.CreatedAt,
	}
}
