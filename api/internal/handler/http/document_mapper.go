package http

import (
	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

func documentView(d model.Document) map[string]any {
	return map[string]any{
		"id": d.ID, "tenant_id": d.TenantID, "project_id": d.ProjectID, "folder_id": d.FolderID, "name": d.Name,
		"mime_type": d.MimeType, "byte_size": d.ByteSize, "created_by": d.CreatedBy, "created_at": d.CreatedAt,
	}
}

func visibleDocumentView(v model.Visible[model.Document]) map[string]any {
	m := documentView(v.Value)
	m["permissions"] = v.Perms
	return m
}
