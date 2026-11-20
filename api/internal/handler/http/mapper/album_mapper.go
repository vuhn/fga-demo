package mapper

import (
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

func AlbumView(a output.Album) map[string]any {
	return map[string]any{
		"id": a.ID, "tenant_id": a.TenantID, "project_id": a.ProjectID, "parent_id": a.ParentID, "name": a.Name, "is_private": a.IsPrivate,
		"created_by": a.CreatedBy, "created_at": a.CreatedAt,
	}
}

func VisibleAlbumView(v output.Visible[output.Album]) map[string]any {
	m := AlbumView(v.Value)
	m["permissions"] = v.Perms
	return m
}
