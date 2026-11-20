package output

import (
	"time"
)

// Photo is the usecase output copy of a photo.
type Photo struct {
	ID        string
	TenantID  string
	ProjectID string
	AlbumID   string
	Name      string
	MimeType  string
	ByteSize  int64
	CreatedBy string
	CreatedAt time.Time
}

// PhotoCreator is the result of PhotoCreator.Create.
type PhotoCreator struct {
	Photo
}

// PhotoGetter is the result of PhotoGetter.Get.
type PhotoGetter struct {
	Visible[Photo]
}

// PhotoGrantLister is the result of PhotoGrantLister.ListGrants.
type PhotoGrantLister struct {
	Items []Grant
}

// PhotoLister is the result of PhotoLister.List.
type PhotoLister struct {
	Items []Visible[Photo]
}

// PhotoRenamer is the result of PhotoRenamer.Rename.
type PhotoRenamer struct {
	Photo
}

// PhotoShareCandidateLister is the result of PhotoShareCandidateLister.ListShareCandidates.
type PhotoShareCandidateLister struct {
	Items []Membership
}

// PhotoSharer is the result of PhotoSharer.Share.
type PhotoSharer struct {
	Grant
}
