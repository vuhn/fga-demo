package output

import (
	"time"
)

// Document is the usecase output copy of a document.
type Document struct {
	ID        string
	TenantID  string
	ProjectID string
	FolderID  string
	Name      string
	MimeType  string
	ByteSize  int64
	CreatedBy string
	CreatedAt time.Time
}

// DocumentCreator is the result of DocumentCreator.Create.
type DocumentCreator struct {
	Document
}

// DocumentGetter is the result of DocumentGetter.Get.
type DocumentGetter struct {
	Visible[Document]
}

// DocumentGrantLister is the result of DocumentGrantLister.ListGrants.
type DocumentGrantLister struct {
	Items []Grant
}

// DocumentLister is the result of DocumentLister.List.
type DocumentLister struct {
	Items []Visible[Document]
}

// DocumentRenamer is the result of DocumentRenamer.Rename.
type DocumentRenamer struct {
	Document
}

// DocumentShareCandidateLister is the result of DocumentShareCandidateLister.ListShareCandidates.
type DocumentShareCandidateLister struct {
	Items []Membership
}

// DocumentSharer is the result of DocumentSharer.Share.
type DocumentSharer struct {
	Grant
}
