package http

import (
	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

func albumView(a model.Album) map[string]any {
	return map[string]any{
		"id": a.ID, "tenant_id": a.TenantID, "project_id": a.ProjectID, "parent_id": a.ParentID, "name": a.Name, "is_private": a.IsPrivate,
		"created_by": a.CreatedBy, "created_at": a.CreatedAt,
	}
}

func visibleAlbumView(v model.Visible[model.Album]) map[string]any {
	m := albumView(v.Value)
	m["permissions"] = v.Perms
	return m
}
