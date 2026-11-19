package gateway

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

type UserGateway interface {
	Create(ctx context.Context, u model.User) error
	Get(ctx context.Context, id string) (model.User, error)
	GetByEmail(ctx context.Context, email string) (model.User, error)
	List(ctx context.Context) ([]model.User, error)
	ListPlatformAdmins(ctx context.Context) ([]model.User, error)
	UpdateName(ctx context.Context, id, name string) (model.User, error)
}
