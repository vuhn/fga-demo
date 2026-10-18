package http

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/vuhn/fga-demo/internal/domain"
	"github.com/vuhn/fga-demo/internal/usecase"
)

type handlers struct{ app *usecase.App }

func NewRouter(app *usecase.App, tokens TokenVerifier, corsOrigin string, log *slog.Logger) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery(), requestLog(log), cors(corsOrigin))

	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	h := &handlers{app: app}
	v1 := r.Group("/api/v1")
	v1.POST("/auth/login", h.login)

	authed := v1.Group("", authenticate(tokens))
	authed.GET("/me", h.me)
	authed.GET("/me/permissions", h.myPermissions)

	authed.GET("/users", h.listUsers)
	authed.POST("/users", h.createUser)
	authed.PATCH("/users/:id", h.updateUser)

	authed.GET("/tenants", h.listTenants)
	authed.POST("/tenants", h.createTenant)
	authed.GET("/tenants/mine", h.myTenants)
	authed.GET("/tenants/:id", h.getTenant)
	authed.PATCH("/tenants/:id", h.updateTenant)
	authed.GET("/tenants/:id/products", h.listProducts)
	authed.PUT("/tenants/:id/products/:product", h.setProduct)
	authed.GET("/tenants/:id/members", h.listMembers)
	authed.POST("/tenants/:id/members", h.addMember)
	authed.PATCH("/tenants/:id/members/:userId", h.changeRole)
	authed.DELETE("/tenants/:id/members/:userId", h.removeMember)

	authed.POST("/authz/check", h.check)
	authed.POST("/authz/batch-check", h.batchCheck)
	authed.GET("/access", h.access)

	tenanted := authed.Group("", requireTenant())
	content{app: app, kind: domain.Documents}.mount(tenanted)
	content{app: app, kind: domain.Photos}.mount(tenanted)

	return r
}
