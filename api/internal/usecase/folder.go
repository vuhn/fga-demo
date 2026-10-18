package usecase

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
)

type FolderLister interface {
	List(ctx context.Context, actor, tenantID string) ([]entity.Visible[entity.Folder], error)
}

type FolderGetter interface {
	Get(ctx context.Context, actor, tenantID, id string) (entity.Visible[entity.Folder], error)
}

type CreateFolderInput struct {
	Name      string
	ParentID  *string
	IsPrivate bool
}

type FolderCreator interface {
	Create(ctx context.Context, actor, tenantID string, in CreateFolderInput) (entity.Folder, error)
}

type FolderRenamer interface {
	Rename(ctx context.Context, actor, tenantID, id, name string) (entity.Folder, error)
}

type FolderVisibilitySetter interface {
	SetVisibility(ctx context.Context, actor, tenantID, id string, isPrivate bool) (entity.Folder, error)
}

type FolderDeleter interface {
	Delete(ctx context.Context, actor, tenantID, id string) error
}

type FolderGrantLister interface {
	ListGrants(ctx context.Context, actor, tenantID, id string) ([]entity.Grant, error)
}

// FolderShareCandidateLister lists the external members a sharer can pick.
type FolderShareCandidateLister interface {
	ListShareCandidates(ctx context.Context, actor, tenantID, id string) ([]entity.Membership, error)
}

type FolderSharer interface {
	Share(ctx context.Context, actor, tenantID, id string, in entity.ShareInput) (entity.Grant, error)
}

type FolderShareRevoker interface {
	Revoke(ctx context.Context, actor, tenantID, id string, in entity.ShareInput) error
}
