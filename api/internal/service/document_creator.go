package service

import (
	"context"
	"strings"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type documentCreator struct {
	documents gateway.DocumentGateway
	folders   gateway.FolderGateway
	relations gateway.DocumentRelationGateway
	authz     helper.Authorizer
	guard     helper.WriteGuard
	ids       gateway.IDGenerator
}

func NewDocumentCreator(documents gateway.DocumentGateway, folders gateway.FolderGateway, relations gateway.DocumentRelationGateway,
	authz helper.Authorizer, guard helper.WriteGuard, ids gateway.IDGenerator) usecase.DocumentCreator {
	return &documentCreator{documents: documents, folders: folders, relations: relations, authz: authz, guard: guard, ids: ids}
}

// Create is the upload command: can_create_document on the folder. It stores
// metadata only.
func (s *documentCreator) Create(ctx context.Context, in input.DocumentCreator) (output.DocumentCreator, error) {
	name, err := model.CleanName("name", in.Name)
	if err != nil {
		return output.DocumentCreator{}, err
	}
	mime := strings.TrimSpace(in.MimeType)
	if n := len(mime); n < 1 || n > 200 {
		return output.DocumentCreator{}, model.Invalid("mime_type length must be 1 to 200")
	}
	if in.ByteSize < 0 {
		return output.DocumentCreator{}, model.Invalid("byte_size must be >= 0")
	}
	folder, err := loadFolder(ctx, s.folders, in.TenantID, in.FolderID)
	if err != nil {
		return output.DocumentCreator{}, err
	}
	if err := s.authz.Require(ctx, in.Actor, "can_create_document", model.ObjectTypeFolder, in.FolderID); err != nil {
		return output.DocumentCreator{}, err
	}

	d := model.Document{ID: s.ids.NewID(), TenantID: in.TenantID, ProjectID: folder.ProjectID, FolderID: in.FolderID, Name: name, MimeType: mime, ByteSize: in.ByteSize, CreatedBy: in.Actor}
	owner := model.OwnerGrant(s.ids.NewID(), in.TenantID, model.ObjectTypeDocument, d.ID, in.Actor)
	if err := s.documents.Create(ctx, d, owner); err != nil {
		return output.DocumentCreator{}, err
	}
	err = s.guard.Grant(ctx, s.relations.Created(d), nil, func(ctx context.Context) error {
		return s.documents.Delete(ctx, d.ID)
	})
	if err != nil {
		return output.DocumentCreator{}, err
	}
	got, err := s.documents.Get(ctx, d.ID)
	return output.DocumentCreator{Document: documentOutput(got)}, err
}
