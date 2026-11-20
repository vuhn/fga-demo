package input

// DocumentCreator is the input of DocumentCreator.Create.
type DocumentCreator struct {
	Actor    string
	TenantID string
	FolderID string
	Name     string
	MimeType string
	ByteSize int64
}

// DocumentDeleter is the input of DocumentDeleter.Delete.
type DocumentDeleter struct {
	Actor    string
	TenantID string
	ID       string
}

// DocumentGetter is the input of DocumentGetter.Get.
type DocumentGetter struct {
	Actor    string
	TenantID string
	ID       string
}

// DocumentGrantLister is the input of DocumentGrantLister.ListGrants.
type DocumentGrantLister struct {
	Actor    string
	TenantID string
	ID       string
}

// DocumentLister is the input of DocumentLister.List.
type DocumentLister struct {
	Actor     string
	TenantID  string
	FolderID  string
	ProjectID string
}

// DocumentRenamer is the input of DocumentRenamer.Rename.
type DocumentRenamer struct {
	Actor    string
	TenantID string
	ID       string
	Name     string
}

// DocumentShareCandidateLister is the input of DocumentShareCandidateLister.ListShareCandidates.
type DocumentShareCandidateLister struct {
	Actor    string
	TenantID string
	ID       string
}

// DocumentShareRevoker is the input of DocumentShareRevoker.Revoke.
type DocumentShareRevoker struct {
	Actor    string
	TenantID string
	ID       string
	Relation string
	Subject  string
	UserID   string
}

// DocumentSharer is the input of DocumentSharer.Share.
type DocumentSharer struct {
	Actor    string
	TenantID string
	ID       string
	Relation string
	Subject  string
	UserID   string
}
