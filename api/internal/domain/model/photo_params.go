package model

// ListPhotosParams selects photos by id inside one tenant. AlbumID and
// ProjectID narrow the result when they are not empty.
type ListPhotosParams struct {
	TenantID  string
	IDs       []string
	AlbumID   string
	ProjectID string
}
