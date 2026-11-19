package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
)

// objectTenantResolver reads a content object's tenant from its own gateway.
type objectTenantResolver struct {
	folders    gateway.FolderGateway
	documents  gateway.DocumentGateway
	albums     gateway.AlbumGateway
	photos     gateway.PhotoGateway
	projects   gateway.ProjectGateway
	blueprints gateway.BlueprintGateway
}

func NewObjectTenantResolver(folders gateway.FolderGateway, documents gateway.DocumentGateway, albums gateway.AlbumGateway, photos gateway.PhotoGateway,
	projects gateway.ProjectGateway, blueprints gateway.BlueprintGateway) helper.ObjectTenantResolver {
	return &objectTenantResolver{folders: folders, documents: documents, albums: albums, photos: photos, projects: projects, blueprints: blueprints}
}

func (r *objectTenantResolver) TenantOf(ctx context.Context, objectType, objectID string) (string, error) {
	if err := model.RequireUUID("id", objectID); err != nil {
		return "", err
	}
	switch objectType {
	case model.ObjectTypeFolder:
		f, err := r.folders.Get(ctx, objectID)
		return f.TenantID, err
	case model.ObjectTypeDocument:
		d, err := r.documents.Get(ctx, objectID)
		return d.TenantID, err
	case model.ObjectTypeAlbum:
		a, err := r.albums.Get(ctx, objectID)
		return a.TenantID, err
	case model.ObjectTypeProject:
		p, err := r.projects.Get(ctx, objectID)
		return p.TenantID, err
	case model.ObjectTypePhoto:
		p, err := r.photos.Get(ctx, objectID)
		return p.TenantID, err
	case model.ObjectTypeBlueprint:
		b, err := r.blueprints.Get(ctx, objectID)
		return b.TenantID, err
	}
	return "", model.Invalid("unknown object type %q", objectType)
}
