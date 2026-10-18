package entity

import "time"

// ObjectTypeAlbum is the OpenFGA type and resource_grants.object_type of an album.
const ObjectTypeAlbum = "album"

type Album struct {
	ID       string
	TenantID string
	ParentID *string
	Name     string
	// IsPrivate hides the album from external members unless shared.
	// An album is externally visible only if it and every ancestor are public.
	IsPrivate bool
	CreatedBy string
	CreatedAt time.Time
}
