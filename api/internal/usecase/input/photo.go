package input

// PhotoCreator is the input of PhotoCreator.Create.
type PhotoCreator struct {
	Actor    string
	TenantID string
	AlbumID  string
	Name     string
	MimeType string
	ByteSize int64
}

// PhotoDeleter is the input of PhotoDeleter.Delete.
type PhotoDeleter struct {
	Actor    string
	TenantID string
	ID       string
}

// PhotoGetter is the input of PhotoGetter.Get.
type PhotoGetter struct {
	Actor    string
	TenantID string
	ID       string
}

// PhotoGrantLister is the input of PhotoGrantLister.ListGrants.
type PhotoGrantLister struct {
	Actor    string
	TenantID string
	ID       string
}

// PhotoLister is the input of PhotoLister.List.
type PhotoLister struct {
	Actor     string
	TenantID  string
	AlbumID   string
	ProjectID string
}

// PhotoRenamer is the input of PhotoRenamer.Rename.
type PhotoRenamer struct {
	Actor    string
	TenantID string
	ID       string
	Name     string
}

// PhotoShareCandidateLister is the input of PhotoShareCandidateLister.ListShareCandidates.
type PhotoShareCandidateLister struct {
	Actor    string
	TenantID string
	ID       string
}

// PhotoShareRevoker is the input of PhotoShareRevoker.Revoke.
type PhotoShareRevoker struct {
	Actor    string
	TenantID string
	ID       string
	Relation string
	Subject  string
	UserID   string
}

// PhotoSharer is the input of PhotoSharer.Share.
type PhotoSharer struct {
	Actor    string
	TenantID string
	ID       string
	Relation string
	Subject  string
	UserID   string
}
