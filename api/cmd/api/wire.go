package main

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	httph "github.com/vuhn/fga-demo/api/internal/handler/http"
	"github.com/vuhn/fga-demo/api/internal/infrastructure/auth"
	"github.com/vuhn/fga-demo/api/internal/infrastructure/blueprintengine"
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
		users          = postgres.NewUserRepo(pool)
		tenants        = postgres.NewTenantRepo(pool)
		products       = postgres.NewProductRepo(pool)
		members        = postgres.NewMembershipRepo(pool)
		companies      = postgres.NewCompanyRepo(pool)
		projects       = postgres.NewProjectRepo(pool)
		projectRoles   = postgres.NewProjectRoleRepo(pool)
		projectMembers = postgres.NewProjectMemberRepo(pool)
		roles          = postgres.NewRoleRepo(pool)
		folders        = postgres.NewFolderRepo(pool)
		folderGrant    = postgres.NewFolderGrantRepo(pool)
		documents      = postgres.NewDocumentRepo(pool)
		docGrant       = postgres.NewDocumentGrantRepo(pool)
		albums         = postgres.NewAlbumRepo(pool)
		albumGrant     = postgres.NewAlbumGrantRepo(pool)
		photos         = postgres.NewPhotoRepo(pool)
		photoGrant     = postgres.NewPhotoGrantRepo(pool)
		blueprints     = postgres.NewBlueprintRepo(pool)
		bpGrant        = postgres.NewBlueprintGrantRepo(pool)

		memberRels        = fga.MembershipRelations{}
		roleRels          = fga.RoleRelations{}
		productRels       = fga.ProductRelations{}
		projectRels       = fga.ProjectRelations{}
		projectRoleRels   = fga.ProjectRoleRelations{}
		projectMemberRels = fga.ProjectMemberRelations{}
		folderRels        = fga.FolderRelations{}
		documentRels      = fga.DocumentRelations{}
		albumRels         = fga.AlbumRelations{}
		photoRels         = fga.PhotoRelations{}
		blueprintRels     = fga.BlueprintRelations{}

		engine = blueprintengine.New(cfg.BlueprintEngine)

		tokens = auth.JWT{Secret: []byte(cfg.JWTSecret), TTL: cfg.JWTTTL}
		hasher = auth.Bcrypt{}
		ids    = idgen.UUID{}
	)

	// Shared services
	authz := service.NewAuthorizer(authzStore)
	guard := service.NewWriteGuard(authzStore, log)
	policy := service.NewSharePolicy(authz)
	objectTenants := service.NewObjectTenantResolver(folders, documents, albums, photos, projects, blueprints)
	myTenants := service.NewMyTenantLister(tenants, members, authz)

	handlers := []httph.RouteRegistrar{
		httph.NewAuthHandler(
			service.NewAuthenticator(users, hasher, tokens, authz),
			service.NewProfileGetter(users, authz, myTenants),
		),
		httph.NewPermissionHandler(service.NewPermissionReader(authz)),
		httph.NewUserHandler(
			service.NewUserLister(users, authz),
			service.NewUserCreator(users, hasher, authz, ids, companies),
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
			service.NewMemberAdder(tenants, users, companies, members, roles, memberRels, roleRels, hasher, authz, guard, ids),
			service.NewMemberRoleChanger(members, roles, memberRels, roleRels, authz, guard),
			service.NewMemberRemover(members, memberRels, roles, roleRels, projectMembers, projectMemberRels, projectRoleRels, authz, guard),
		),
		httph.NewCompanyHandler(
			service.NewCompanyLister(tenants, companies, authz),
			service.NewAllCompanyLister(companies, authz),
			service.NewCompanyCreator(tenants, companies, authz, ids),
			service.NewCompanyClaimer(tenants, companies, authz),
		),
		httph.NewProjectHandler(
			service.NewProjectLister(projects, authz),
			service.NewProjectGetter(projects, authz),
			service.NewProjectCreator(tenants, projects, projectRels, authz, guard, ids),
			service.NewProjectRenamer(projects, authz),
		),
		httph.NewProjectRoleHandler(
			service.NewProjectRoleLister(projects, projectRoles, authz),
			service.NewProjectRoleCreator(projects, projectRoles, projectRoleRels, authz, guard, ids),
			service.NewProjectRolePermissionSetter(projects, projectRoles, projectRoleRels, authz, guard),
			service.NewProjectRoleDeleter(projects, projectRoles, projectRoleRels, authz, guard),
		),
		httph.NewProjectMemberHandler(
			service.NewProjectMemberLister(projects, projectMembers, authz),
			service.NewProjectMemberCandidateLister(projects, members, projectMembers, authz),
			service.NewProjectMemberAdder(projects, members, projectMembers, projectRoles, projectMemberRels, projectRoleRels, authz, guard),
			service.NewProjectMemberRoleChanger(projects, projectMembers, projectRoles, projectMemberRels, projectRoleRels, authz, guard),
			service.NewProjectMemberRemover(projects, projectMembers, projectMemberRels, projectRoleRels, authz, guard),
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
			Creator:          service.NewFolderCreator(projects, folders, folderRels, authz, guard, ids),
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
			Creator:          service.NewAlbumCreator(projects, albums, albumRels, authz, guard, ids),
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
		httph.NewBlueprintHandler(httph.BlueprintUsecases{
			Lister:          service.NewBlueprintLister(blueprints, authz),
			Getter:          service.NewBlueprintGetter(blueprints, authz),
			Drafter:         service.NewBlueprintDrafter(projects, engine, authz),
			SketchDrafter:   service.NewBlueprintSketchDrafter(projects, engine, authz),
			Creator:         service.NewBlueprintCreator(projects, blueprints, blueprintRels, engine, authz, guard, ids),
			Reviser:         service.NewBlueprintReviser(blueprints, engine, authz),
			Reviewer:        service.NewBlueprintReviewer(blueprints, engine, authz),
			BriefUpdater:    service.NewBlueprintBriefUpdater(blueprints, engine, authz),
			VersionLister:   service.NewBlueprintVersionLister(blueprints, authz),
			Restorer:        service.NewBlueprintVersionRestorer(blueprints, authz),
			Renderer:        service.NewBlueprintDrawingRenderer(blueprints, engine, authz),
			Renamer:         service.NewBlueprintRenamer(blueprints, authz),
			Deleter:         service.NewBlueprintDeleter(blueprints, authz, guard),
			GrantLister:     service.NewBlueprintGrantLister(blueprints, bpGrant, authz),
			CandidateLister: service.NewBlueprintShareCandidateLister(blueprints, members, authz),
			Sharer:          service.NewBlueprintSharer(blueprints, bpGrant, blueprintRels, authz, policy, guard, ids),
			ShareRevoker:    service.NewBlueprintShareRevoker(blueprints, bpGrant, blueprintRels, authz, policy, guard),
		}),
	}
	return httph.NewRouter(service.NewTokenVerifier(tokens), cfg.CORSOrigin, log, handlers...)
}
