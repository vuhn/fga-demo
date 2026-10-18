package entity

import "time"

// ObjectTypeFolder is the OpenFGA type and resource_grants.object_type of a folder.
const ObjectTypeFolder = "folder"

type Folder struct {
	ID       string
	TenantID string
	ParentID *string
	Name     string
	// IsPrivate hides the folder from external members unless shared.
	// A folder is externally visible only if it and every ancestor are public.
	IsPrivate bool
	CreatedBy string
	CreatedAt time.Time
}
