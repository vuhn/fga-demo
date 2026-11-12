package service

import (
	"context"
	"strings"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
)

type memberAdder struct {
	tenants   gateway.TenantGateway
	users     gateway.UserGateway
	companies gateway.CompanyGateway
	members   gateway.MembershipGateway
	roles     gateway.RoleGateway
	relations gateway.MembershipRelationGateway
	roleRels  gateway.RoleRelationGateway
	hasher    gateway.PasswordHasher
	authz     usecase.Authorizer
	guard     helper.WriteGuard
	ids       gateway.IDGenerator
}

func NewMemberAdder(tenants gateway.TenantGateway, users gateway.UserGateway, companies gateway.CompanyGateway, members gateway.MembershipGateway, roles gateway.RoleGateway,
	relations gateway.MembershipRelationGateway, roleRels gateway.RoleRelationGateway, hasher gateway.PasswordHasher, authz usecase.Authorizer, guard helper.WriteGuard, ids gateway.IDGenerator) usecase.MemberAdder {
	return &memberAdder{tenants: tenants, users: users, companies: companies, members: members, roles: roles, relations: relations, roleRels: roleRels, hasher: hasher, authz: authz, guard: guard, ids: ids}
}

// Add makes a person a tenant member. The person is named by user id or by
// email; one email is one account, so a known email reuses the account and an
// unknown one creates it in a company the tenant manages. Affiliation is not
// an input: it is internal when the user's company is the tenant's own.
func (s *memberAdder) Add(ctx context.Context, actor, tenantID string, in usecase.AddMemberInput) (entity.Membership, error) {
	if err := s.authz.PlatformOr(ctx, actor, "can_manage_members", tenantID, entity.ErrForbidden); err != nil {
		return entity.Membership{}, err
	}
	if _, err := s.tenants.Get(ctx, tenantID); err != nil {
		return entity.Membership{}, err
	}
	user, err := s.resolveUser(ctx, actor, tenantID, in)
	if err != nil {
		return entity.Membership{}, err
	}
	company, err := s.companies.Get(ctx, user.CompanyID)
	if err != nil {
		return entity.Membership{}, err
	}
	affiliation := entity.DeriveAffiliation(company, tenantID)
	assigned, err := assignMemberRole(ctx, s.roles, s.relations, s.roleRels, tenantID, user.ID, affiliation, in.Role, "")
	if err != nil {
		return entity.Membership{}, err
	}

	m := entity.Membership{TenantID: tenantID, UserID: user.ID, Role: assigned.StoredRole, CustomRoleID: assigned.CustomRoleID}
	if err := s.members.Insert(ctx, m); err != nil {
		return entity.Membership{}, err
	}
	err = s.guard.Grant(ctx, assigned.Writes, assigned.Deletes, func(ctx context.Context) error {
		return s.members.Delete(ctx, tenantID, user.ID)
	})
	if err != nil {
		return entity.Membership{}, err
	}
	return s.members.Get(ctx, tenantID, user.ID)
}

func (s *memberAdder) resolveUser(ctx context.Context, actor, tenantID string, in usecase.AddMemberInput) (entity.User, error) {
	if in.UserID != "" {
		if err := entity.RequireUUID("user_id", in.UserID); err != nil {
			return entity.User{}, err
		}
		return s.users.Get(ctx, in.UserID)
	}
	email := strings.TrimSpace(in.Email)
	if !strings.Contains(email, "@") {
		return entity.User{}, entity.Invalid("user_id or a valid email is required")
	}
	if u, err := s.users.GetByEmail(ctx, email); err == nil {
		return u, nil // one email, one account
	}
	name, err := entity.CleanName("name", in.Name)
	if err != nil {
		return entity.User{}, err
	}
	if n := len(in.Password); n < 8 || n > 72 {
		return entity.User{}, entity.Invalid("password length must be 8 to 72")
	}
	if err := entity.RequireUUID("company_id", in.CompanyID); err != nil {
		return entity.User{}, err
	}
	if err := s.requireUsableCompany(ctx, actor, tenantID, in.CompanyID); err != nil {
		return entity.User{}, err
	}
	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return entity.User{}, err
	}
	u := entity.User{ID: s.ids.NewID(), Email: email, Name: name, PasswordHash: hash, CompanyID: in.CompanyID}
	if err := s.users.Create(ctx, u); err != nil {
		return entity.User{}, err
	}
	return s.users.Get(ctx, u.ID)
}

// requireUsableCompany limits a tenant to its own company and the companies it
// created. A platform admin may use any company.
func (s *memberAdder) requireUsableCompany(ctx context.Context, actor, tenantID, companyID string) error {
	if ok, err := s.authz.IsPlatformAdmin(ctx, actor); err != nil || ok {
		return err
	}
	cs, err := s.companies.ListForTenant(ctx, tenantID)
	if err != nil {
		return err
	}
	for _, c := range cs {
		if c.ID == companyID {
			return nil
		}
	}
	return entity.ErrNotFound
}
