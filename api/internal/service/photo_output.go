package service

import (
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

func photoOutput(m model.Photo) output.Photo {
	return output.Photo{
		ID:        m.ID,
		TenantID:  m.TenantID,
		ProjectID: m.ProjectID,
		AlbumID:   m.AlbumID,
		Name:      m.Name,
		MimeType:  m.MimeType,
		ByteSize:  m.ByteSize,
		CreatedBy: m.CreatedBy,
		CreatedAt: m.CreatedAt,
	}
}
