package entity

import "time"

// ObjectTypeDocument is the OpenFGA type and resource_grants.object_type of a document.
const ObjectTypeDocument = "document"

// Document is metadata only; there are no file bytes.
type Document struct {
	ID        string
	TenantID  string
	FolderID  string
	Name      string
	MimeType  string
	ByteSize  int64
	CreatedBy string
	CreatedAt time.Time
}
