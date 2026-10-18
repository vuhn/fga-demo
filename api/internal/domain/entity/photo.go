package entity

import "time"

// ObjectTypePhoto is the OpenFGA type and resource_grants.object_type of a photo.
const ObjectTypePhoto = "photo"

// Photo is metadata only; there are no file bytes.
type Photo struct {
	ID        string
	TenantID  string
	AlbumID   string
	Name      string
	MimeType  string
	ByteSize  int64
	CreatedBy string
	CreatedAt time.Time
}
