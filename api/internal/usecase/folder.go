package usecase

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

type FolderLister interface {
	// List returns visible folders; projectID narrows to one project when not empty.
	List(ctx context.Context, actor, tenantID, projectID string) ([]model.Visible[model.Folder], error)
}

type FolderGetter interface {
	Get(ctx context.Context, actor, tenantID, id string) (model.Visible[model.Folder], error)
}

type CreateFolderInput struct {
	// ProjectID is required for a top-level folder; a subfolder takes its parent's.
	ProjectID string
	Name      string
	ParentID  *string
	IsPrivate bool
}

type FolderCreator interface {
	Create(ctx context.Context, actor, tenantID string, in CreateFolderInput) (model.Folder, error)
}

type FolderRenamer interface {
	Rename(ctx context.Context, actor, tenantID, id, name string) (model.Folder, error)
}

type FolderVisibilitySetter interface {
	SetVisibility(ctx context.Context, actor, tenantID, id string, isPrivate bool) (model.Folder, error)
}

type FolderDeleter interface {
	Delete(ctx context.Context, actor, tenantID, id string) error
}

type FolderGrantLister interface {
	ListGrants(ctx context.Context, actor, tenantID, id string) ([]model.Grant, error)
}

// FolderShareCandidateLister lists the external members a sharer can pick.
type FolderShareCandidateLister interface {
	ListShareCandidates(ctx context.Context, actor, tenantID, id string) ([]model.Membership, error)
}

type FolderSharer interface {
	Share(ctx context.Context, actor, tenantID, id string, in model.ShareInput) (model.Grant, error)
}

type FolderShareRevoker interface {
	Revoke(ctx context.Context, actor, tenantID, id string, in model.ShareInput) error
}
