package output

import (
	"time"
)

// Folder is the usecase output copy of a folder.
type Folder struct {
	ID        string
	TenantID  string
	ProjectID string
	ParentID  *string
	Name      string
	IsPrivate bool
	CreatedBy string
	CreatedAt time.Time
}

// FolderCreator is the result of FolderCreator.Create.
type FolderCreator struct {
	Folder
}

// FolderGetter is the result of FolderGetter.Get.
type FolderGetter struct {
	Visible[Folder]
}

// FolderGrantLister is the result of FolderGrantLister.ListGrants.
type FolderGrantLister struct {
	Items []Grant
}

// FolderLister is the result of FolderLister.List.
type FolderLister struct {
	Items []Visible[Folder]
}

// FolderRenamer is the result of FolderRenamer.Rename.
type FolderRenamer struct {
	Folder
}

// FolderShareCandidateLister is the result of FolderShareCandidateLister.ListShareCandidates.
type FolderShareCandidateLister struct {
	Items []Membership
}

// FolderSharer is the result of FolderSharer.Share.
type FolderSharer struct {
	Grant
}

// FolderVisibilitySetter is the result of FolderVisibilitySetter.SetVisibility.
type FolderVisibilitySetter struct {
	Folder
}
