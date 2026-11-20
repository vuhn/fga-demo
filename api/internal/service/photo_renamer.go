package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type photoRenamer struct {
	photos gateway.PhotoGateway
	authz  helper.Authorizer
}

func NewPhotoRenamer(photos gateway.PhotoGateway, authz helper.Authorizer) usecase.PhotoRenamer {
	return &photoRenamer{photos: photos, authz: authz}
}

// Rename requires can_edit.
func (s *photoRenamer) Rename(ctx context.Context, in input.PhotoRenamer) (output.PhotoRenamer, error) {
	if _, err := loadPhoto(ctx, s.photos, in.TenantID, in.ID); err != nil {
		return output.PhotoRenamer{}, err
	}
	name, err := model.CleanName("name", in.Name)
	if err != nil {
		return output.PhotoRenamer{}, err
	}
	if err := s.authz.Require(ctx, in.Actor, "can_edit", model.ObjectTypePhoto, in.ID); err != nil {
		return output.PhotoRenamer{}, err
	}
	got, err := s.photos.Rename(ctx, in.ID, name)
	return output.PhotoRenamer{Photo: photoOutput(got)}, err
}
