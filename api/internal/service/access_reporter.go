package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
)

type accessReporter struct {
	authzStore gateway.AuthzGateway
	members    gateway.MembershipGateway
	users      gateway.UserGateway
	authz      usecase.Authorizer
	tenants    helper.ObjectTenantResolver
}

func NewAccessReporter(authzStore gateway.AuthzGateway, members gateway.MembershipGateway, users gateway.UserGateway,
	authz usecase.Authorizer, tenants helper.ObjectTenantResolver) usecase.AccessReporter {
	return &accessReporter{authzStore: authzStore, members: members, users: users, authz: authz, tenants: tenants}
}

var contentTypes = map[string]bool{
	entity.ObjectTypeFolder: true, entity.ObjectTypeDocument: true,
	entity.ObjectTypeAlbum: true, entity.ObjectTypePhoto: true,
}

// Report lists the direct tuples on an object and evaluates every tenant
// member plus every platform admin with BatchCheck. The platform, a sharer
// of the object, or the tenant's member manager may ask.
func (s *accessReporter) Report(ctx context.Context, actor, objectType, objectID string) (usecase.AccessReport, error) {
	if !contentTypes[objectType] {
		return usecase.AccessReport{}, entity.Invalid("object_type must be folder, document, album, or photo")
	}
	tenantID, err := s.tenants.TenantOf(ctx, objectType, objectID)
	if err != nil {
		return usecase.AccessReport{}, err
	}
	if err := s.requireReader(ctx, actor, objectType, objectID, tenantID); err != nil {
		return usecase.AccessReport{}, err
	}

	direct, err := s.authzStore.ReadTuples(ctx, objectType, objectID)
	if err != nil {
		return usecase.AccessReport{}, err
	}
	people, err := s.people(ctx, tenantID)
	if err != nil {
		return usecase.AccessReport{}, err
	}
	rels := []string{"can_view", "can_edit", "can_delete"}
	var checks []entity.CheckRequest
	for _, p := range people {
		for _, r := range rels {
			checks = append(checks, entity.CheckRequest{UserID: p.UserID, Relation: r, ObjectType: objectType, ObjectID: objectID})
		}
	}
	res, err := s.authz.BatchCheck(ctx, checks)
	if err != nil {
		return usecase.AccessReport{}, err
	}
	for i := range people {
		people[i].CanView, people[i].CanEdit, people[i].CanDelete = res[3*i], res[3*i+1], res[3*i+2]
	}
	if direct == nil {
		direct = []entity.Tuple{}
	}
	return usecase.AccessReport{Object: objectType + ":" + objectID, DirectTuples: direct, Members: people}, nil
}

func (s *accessReporter) requireReader(ctx context.Context, actor, objectType, objectID, tenantID string) error {
	for _, c := range []entity.CheckRequest{
		{UserID: actor, Relation: "can_manage_platform", ObjectType: entity.ObjectTypePlatform, ObjectID: entity.PlatformID},
		{UserID: actor, Relation: "can_share", ObjectType: objectType, ObjectID: objectID},
		{UserID: actor, Relation: "can_manage_members", ObjectType: entity.ObjectTypeTenant, ObjectID: tenantID},
	} {
		ok, err := s.authz.Check(ctx, c.UserID, c.Relation, c.ObjectType, c.ObjectID)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
	}
	return entity.Denied(entity.ErrForbidden, "can_share", objectType, objectID)
}

// people is every tenant member, then every platform admin not already listed.
func (s *accessReporter) people(ctx context.Context, tenantID string) ([]usecase.MemberAccess, error) {
	members, err := s.members.List(ctx, tenantID, "")
	if err != nil {
		return nil, err
	}
	admins, err := s.users.ListPlatformAdmins(ctx)
	if err != nil {
		return nil, err
	}
	people := []usecase.MemberAccess{}
	seen := map[string]bool{}
	for _, m := range members {
		people = append(people, usecase.MemberAccess{UserID: m.UserID, Name: m.Name})
		seen[m.UserID] = true
	}
	for _, u := range admins {
		if !seen[u.ID] {
			people = append(people, usecase.MemberAccess{UserID: u.ID, Name: u.Name})
		}
	}
	return people, nil
}
