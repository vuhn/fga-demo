package input

// FolderCreator is the input of FolderCreator.Create.
type FolderCreator struct {
	Actor     string
	TenantID  string
	ProjectID string
	Name      string
	ParentID  *string
	IsPrivate bool
}

// FolderDeleter is the input of FolderDeleter.Delete.
type FolderDeleter struct {
	Actor    string
	TenantID string
	ID       string
}

// FolderGetter is the input of FolderGetter.Get.
type FolderGetter struct {
	Actor    string
	TenantID string
	ID       string
}

// FolderGrantLister is the input of FolderGrantLister.ListGrants.
type FolderGrantLister struct {
	Actor    string
	TenantID string
	ID       string
}

// FolderLister is the input of FolderLister.List.
type FolderLister struct {
	Actor     string
	TenantID  string
	ProjectID string
}

// FolderRenamer is the input of FolderRenamer.Rename.
type FolderRenamer struct {
	Actor    string
	TenantID string
	ID       string
	Name     string
}

// FolderShareCandidateLister is the input of FolderShareCandidateLister.ListShareCandidates.
type FolderShareCandidateLister struct {
	Actor    string
	TenantID string
	ID       string
}

// FolderShareRevoker is the input of FolderShareRevoker.Revoke.
type FolderShareRevoker struct {
	Actor    string
	TenantID string
	ID       string
	Relation string
	Subject  string
	UserID   string
}

// FolderSharer is the input of FolderSharer.Share.
type FolderSharer struct {
	Actor    string
	TenantID string
	ID       string
	Relation string
	Subject  string
	UserID   string
}

// FolderVisibilitySetter is the input of FolderVisibilitySetter.SetVisibility.
type FolderVisibilitySetter struct {
	Actor     string
	TenantID  string
	ID        string
	IsPrivate bool
}
