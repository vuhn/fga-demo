package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
)

// objectTenantResolver reads a content object's tenant from its own gateway.
type objectTenantResolver struct {
	folders   gateway.FolderGateway
	documents gateway.DocumentGateway
	albums    gateway.AlbumGateway
	photos    gateway.PhotoGateway
}

func NewObjectTenantResolver(folders gateway.FolderGateway, documents gateway.DocumentGateway, albums gateway.AlbumGateway, photos gateway.PhotoGateway) helper.ObjectTenantResolver {
	return &objectTenantResolver{folders: folders, documents: documents, albums: albums, photos: photos}
}

func (r *objectTenantResolver) TenantOf(ctx context.Context, objectType, objectID string) (string, error) {
	if err := entity.RequireUUID("id", objectID); err != nil {
		return "", err
	}
	switch objectType {
	case entity.ObjectTypeFolder:
		f, err := r.folders.Get(ctx, objectID)
		return f.TenantID, err
	case entity.ObjectTypeDocument:
		d, err := r.documents.Get(ctx, objectID)
		return d.TenantID, err
	case entity.ObjectTypeAlbum:
		a, err := r.albums.Get(ctx, objectID)
		return a.TenantID, err
	case entity.ObjectTypePhoto:
		p, err := r.photos.Get(ctx, objectID)
		return p.TenantID, err
	}
	return "", entity.Invalid("unknown object type %q", objectType)
}
