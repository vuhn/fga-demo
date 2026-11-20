package output

import (
	"time"
)

// Album is the usecase output copy of a album.
type Album struct {
	ID        string
	TenantID  string
	ProjectID string
	ParentID  *string
	Name      string
	IsPrivate bool
	CreatedBy string
	CreatedAt time.Time
}

// AlbumCreator is the result of AlbumCreator.Create.
type AlbumCreator struct {
	Album
}

// AlbumGetter is the result of AlbumGetter.Get.
type AlbumGetter struct {
	Visible[Album]
}

// AlbumGrantLister is the result of AlbumGrantLister.ListGrants.
type AlbumGrantLister struct {
	Items []Grant
}

// AlbumLister is the result of AlbumLister.List.
type AlbumLister struct {
	Items []Visible[Album]
}

// AlbumRenamer is the result of AlbumRenamer.Rename.
type AlbumRenamer struct {
	Album
}

// AlbumShareCandidateLister is the result of AlbumShareCandidateLister.ListShareCandidates.
type AlbumShareCandidateLister struct {
	Items []Membership
}

// AlbumSharer is the result of AlbumSharer.Share.
type AlbumSharer struct {
	Grant
}

// AlbumVisibilitySetter is the result of AlbumVisibilitySetter.SetVisibility.
type AlbumVisibilitySetter struct {
	Album
}
