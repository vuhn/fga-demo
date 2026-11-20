package mapper

import (
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

func FolderView(f output.Folder) map[string]any {
	return map[string]any{
		"id": f.ID, "tenant_id": f.TenantID, "project_id": f.ProjectID, "parent_id": f.ParentID, "name": f.Name, "is_private": f.IsPrivate,
		"created_by": f.CreatedBy, "created_at": f.CreatedAt,
	}
}

func VisibleFolderView(v output.Visible[output.Folder]) map[string]any {
	m := FolderView(v.Value)
	m["permissions"] = v.Perms
	return m
}

// VisibilityBody is the folder and album visibility request body.
type VisibilityBody struct {
	IsPrivate *bool `json:"is_private"`
}
