package input

// AlbumCreator is the input of AlbumCreator.Create.
type AlbumCreator struct {
	Actor     string
	TenantID  string
	Name      string
	ProjectID string
	ParentID  *string
	IsPrivate bool
}

// AlbumDeleter is the input of AlbumDeleter.Delete.
type AlbumDeleter struct {
	Actor    string
	TenantID string
	ID       string
}

// AlbumGetter is the input of AlbumGetter.Get.
type AlbumGetter struct {
	Actor    string
	TenantID string
	ID       string
}

// AlbumGrantLister is the input of AlbumGrantLister.ListGrants.
type AlbumGrantLister struct {
	Actor    string
	TenantID string
	ID       string
}

// AlbumLister is the input of AlbumLister.List.
type AlbumLister struct {
	Actor     string
	TenantID  string
	ProjectID string
}

// AlbumRenamer is the input of AlbumRenamer.Rename.
type AlbumRenamer struct {
	Actor    string
	TenantID string
	ID       string
	Name     string
}

// AlbumShareCandidateLister is the input of AlbumShareCandidateLister.ListShareCandidates.
type AlbumShareCandidateLister struct {
	Actor    string
	TenantID string
	ID       string
}

// AlbumShareRevoker is the input of AlbumShareRevoker.Revoke.
type AlbumShareRevoker struct {
	Actor    string
	TenantID string
	ID       string
	Relation string
	Subject  string
	UserID   string
}

// AlbumSharer is the input of AlbumSharer.Share.
type AlbumSharer struct {
	Actor    string
	TenantID string
	ID       string
	Relation string
	Subject  string
	UserID   string
}

// AlbumVisibilitySetter is the input of AlbumVisibilitySetter.SetVisibility.
type AlbumVisibilitySetter struct {
	Actor     string
	TenantID  string
	ID        string
	IsPrivate bool
}
