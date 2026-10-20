package main

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	httph "github.com/vuhn/fga-demo/api/internal/handler/http"
	"github.com/vuhn/fga-demo/api/internal/infrastructure/auth"
	"github.com/vuhn/fga-demo/api/internal/infrastructure/config"
	"github.com/vuhn/fga-demo/api/internal/infrastructure/fga"
	"github.com/vuhn/fga-demo/api/internal/infrastructure/idgen"
	"github.com/vuhn/fga-demo/api/internal/infrastructure/postgres"
	"github.com/vuhn/fga-demo/api/internal/service"
)

// wire builds the object graph by hand: gateways, then services, then
// handlers. Every dependency is passed through a constructor.
func wire(cfg config.Config, pool *pgxpool.Pool, authzStore gateway.AuthzGateway, log *slog.Logger) *gin.Engine {
	// Gateways
	var (
		users       = postgres.NewUserRepo(pool)
		tenants     = postgres.NewTenantRepo(pool)
		products    = postgres.NewProductRepo(pool)
		members     = postgres.NewMembershipRepo(pool)
		roles       = postgres.NewRoleRepo(pool)
		folders     = postgres.NewFolderRepo(pool)
		folderGrant = postgres.NewFolderGrantRepo(pool)
		documents   = postgres.NewDocumentRepo(pool)
		docGrant    = postgres.NewDocumentGrantRepo(pool)
		albums      = postgres.NewAlbumRepo(pool)
		albumGrant  = postgres.NewAlbumGrantRepo(pool)
		photos      = postgres.NewPhotoRepo(pool)
		photoGrant  = postgres.NewPhotoGrantRepo(pool)

		memberRels   = fga.MembershipRelations{}
		roleRels     = fga.RoleRelations{}
		productRels  = fga.ProductRelations{}
		folderRels   = fga.FolderRelations{}
		documentRels = fga.DocumentRelations{}
		albumRels    = fga.AlbumRelations{}
		photoRels    = fga.PhotoRelations{}

		tokens = auth.JWT{Secret: []byte(cfg.JWTSecret), TTL: cfg.JWTTTL}
		hasher = auth.Bcrypt{}
		ids    = idgen.UUID{}
	)

	// Shared services
	authz := service.NewAuthorizer(authzStore)
	guard := service.NewWriteGuard(authzStore, log)
	policy := service.NewSharePolicy(authz)
	objectTenants := service.NewObjectTenantResolver(folders, documents, albums, photos)
	myTenants := service.NewMyTenantLister(tenants, members, authz)

	handlers := []httph.RouteRegistrar{
		httph.NewAuthHandler(
			service.NewAuthenticator(users, hasher, tokens, authz),
			service.NewProfileGetter(users, authz, myTenants),
		),
		httph.NewPermissionHandler(service.NewPermissionReader(authz)),
		httph.NewUserHandler(
			service.NewUserLister(users, authz),
			service.NewUserCreator(users, hasher, authz, ids),
			service.NewUserUpdater(users, authz),
		),
		httph.NewTenantHandler(
			service.NewTenantLister(tenants, authz),
			myTenants,
			service.NewTenantCreator(tenants, authz, ids),
			service.NewTenantGetter(tenants, authz),
			service.NewTenantUpdater(tenants, authz),
		),
		httph.NewProductHandler(
			service.NewProductLister(tenants, products, authz),
			service.NewProductSetter(tenants, products, productRels, authz, guard),
		),
		httph.NewMemberHandler(
			service.NewMemberLister(tenants, members, authz),
			service.NewMemberAdder(tenants, users, members, roles, memberRels, roleRels, authz, guard),
			service.NewMemberRoleChanger(members, roles, memberRels, roleRels, authz, guard),
			service.NewMemberRemover(members, memberRels, roles, roleRels, authz, guard),
		),
		httph.NewRoleHandler(
			service.NewRoleLister(tenants, roles, authz),
			service.NewRoleGetter(roles, authz),
			service.NewRoleCreator(tenants, roles, roleRels, authz, guard, ids),
			service.NewRoleUpdater(roles, authz),
			service.NewRolePermissionSetter(roles, roleRels, authz, guard),
			service.NewRoleDeleter(roles, roleRels, authz, guard),
		),
		httph.NewInspectorHandler(service.NewInspector(authz, objectTenants)),
		httph.NewAccessHandler(service.NewAccessReporter(authzStore, members, users, authz, objectTenants)),
		httph.NewFolderHandler(httph.FolderUsecases{
			Lister:           service.NewFolderLister(folders, authz),
			Getter:           service.NewFolderGetter(folders, authz),
			Creator:          service.NewFolderCreator(folders, folderRels, authz, guard, ids),
			Renamer:          service.NewFolderRenamer(folders, authz),
			VisibilitySetter: service.NewFolderVisibilitySetter(folders, folderRels, authz, guard),
			Deleter:          service.NewFolderDeleter(folders, authz, guard),
			GrantLister:      service.NewFolderGrantLister(folders, folderGrant, authz),
			CandidateLister:  service.NewFolderShareCandidateLister(folders, members, authz),
			Sharer:           service.NewFolderSharer(folders, folderGrant, folderRels, authz, policy, guard, ids),
			ShareRevoker:     service.NewFolderShareRevoker(folders, folderGrant, folderRels, authz, policy, guard),
		}),
		httph.NewDocumentHandler(httph.DocumentUsecases{
			Lister:          service.NewDocumentLister(documents, authz),
			Getter:          service.NewDocumentGetter(documents, authz),
			Creator:         service.NewDocumentCreator(documents, folders, documentRels, authz, guard, ids),
			Renamer:         service.NewDocumentRenamer(documents, authz),
			Deleter:         service.NewDocumentDeleter(documents, authz, guard),
			GrantLister:     service.NewDocumentGrantLister(documents, docGrant, authz),
			CandidateLister: service.NewDocumentShareCandidateLister(documents, members, authz),
			Sharer:          service.NewDocumentSharer(documents, docGrant, documentRels, authz, policy, guard, ids),
			ShareRevoker:    service.NewDocumentShareRevoker(documents, docGrant, documentRels, authz, policy, guard),
		}),
		httph.NewAlbumHandler(httph.AlbumUsecases{
			Lister:           service.NewAlbumLister(albums, authz),
			Getter:           service.NewAlbumGetter(albums, authz),
			Creator:          service.NewAlbumCreator(albums, albumRels, authz, guard, ids),
			Renamer:          service.NewAlbumRenamer(albums, authz),
			VisibilitySetter: service.NewAlbumVisibilitySetter(albums, albumRels, authz, guard),
			Deleter:          service.NewAlbumDeleter(albums, authz, guard),
			GrantLister:      service.NewAlbumGrantLister(albums, albumGrant, authz),
			CandidateLister:  service.NewAlbumShareCandidateLister(albums, members, authz),
			Sharer:           service.NewAlbumSharer(albums, albumGrant, albumRels, authz, policy, guard, ids),
			ShareRevoker:     service.NewAlbumShareRevoker(albums, albumGrant, albumRels, authz, policy, guard),
		}),
		httph.NewPhotoHandler(httph.PhotoUsecases{
			Lister:          service.NewPhotoLister(photos, authz),
			Getter:          service.NewPhotoGetter(photos, authz),
			Creator:         service.NewPhotoCreator(photos, albums, photoRels, authz, guard, ids),
			Renamer:         service.NewPhotoRenamer(photos, authz),
			Deleter:         service.NewPhotoDeleter(photos, authz, guard),
			GrantLister:     service.NewPhotoGrantLister(photos, photoGrant, authz),
			CandidateLister: service.NewPhotoShareCandidateLister(photos, members, authz),
			Sharer:          service.NewPhotoSharer(photos, photoGrant, photoRels, authz, policy, guard, ids),
			ShareRevoker:    service.NewPhotoShareRevoker(photos, photoGrant, photoRels, authz, policy, guard),
		}),
	}
	return httph.NewRouter(service.NewTokenVerifier(tokens), cfg.CORSOrigin, log, handlers...)
}
