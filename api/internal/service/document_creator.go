package service

import (
	"context"
	"strings"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
)

type documentCreator struct {
	documents gateway.DocumentGateway
	folders   gateway.FolderGateway
	relations gateway.DocumentRelationGateway
	authz     usecase.Authorizer
	guard     helper.WriteGuard
	ids       gateway.IDGenerator
}

func NewDocumentCreator(documents gateway.DocumentGateway, folders gateway.FolderGateway, relations gateway.DocumentRelationGateway,
	authz usecase.Authorizer, guard helper.WriteGuard, ids gateway.IDGenerator) usecase.DocumentCreator {
	return &documentCreator{documents: documents, folders: folders, relations: relations, authz: authz, guard: guard, ids: ids}
}

// Create is the upload command: can_create_document on the folder. It stores
// metadata only.
func (s *documentCreator) Create(ctx context.Context, actor, tenantID string, in usecase.CreateDocumentInput) (model.Document, error) {
	name, err := model.CleanName("name", in.Name)
	if err != nil {
		return model.Document{}, err
	}
	mime := strings.TrimSpace(in.MimeType)
	if n := len(mime); n < 1 || n > 200 {
		return model.Document{}, model.Invalid("mime_type length must be 1 to 200")
	}
	if in.ByteSize < 0 {
		return model.Document{}, model.Invalid("byte_size must be >= 0")
	}
	folder, err := loadFolder(ctx, s.folders, tenantID, in.FolderID)
	if err != nil {
		return model.Document{}, err
	}
	if err := s.authz.Require(ctx, actor, "can_create_document", model.ObjectTypeFolder, in.FolderID); err != nil {
		return model.Document{}, err
	}

	d := model.Document{ID: s.ids.NewID(), TenantID: tenantID, ProjectID: folder.ProjectID, FolderID: in.FolderID, Name: name, MimeType: mime, ByteSize: in.ByteSize, CreatedBy: actor}
	owner := model.OwnerGrant(s.ids.NewID(), tenantID, model.ObjectTypeDocument, d.ID, actor)
	if err := s.documents.Create(ctx, d, owner); err != nil {
		return model.Document{}, err
	}
	err = s.guard.Grant(ctx, s.relations.Created(d), nil, func(ctx context.Context) error {
		return s.documents.Delete(ctx, d.ID)
	})
	if err != nil {
		return model.Document{}, err
	}
	return s.documents.Get(ctx, d.ID)
}
