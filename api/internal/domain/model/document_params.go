package model

// ListDocumentsParams selects documents by id inside one tenant. FolderID and
// ProjectID narrow the result when they are not empty.
type ListDocumentsParams struct {
	TenantID  string
	IDs       []string
	FolderID  string
	ProjectID string
}
