package mapper

import (
	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

func PhotoView(p model.Photo) map[string]any {
	return map[string]any{
		"id": p.ID, "tenant_id": p.TenantID, "project_id": p.ProjectID, "album_id": p.AlbumID, "name": p.Name,
		"mime_type": p.MimeType, "byte_size": p.ByteSize, "created_by": p.CreatedBy, "created_at": p.CreatedAt,
	}
}

func VisiblePhotoView(v model.Visible[model.Photo]) map[string]any {
	m := PhotoView(v.Value)
	m["permissions"] = v.Perms
	return m
}
