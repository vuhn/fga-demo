package http

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/vuhn/fga-demo/api/internal/usecase"
)

// Routes are the groups a handler mounts on.
type Routes struct {
	Public   *gin.RouterGroup // /api/v1, no token
	Authed   *gin.RouterGroup // bearer token required
	Tenanted *gin.RouterGroup // bearer token and X-Tenant-ID required
}

// RouteRegistrar is implemented by every handler.
type RouteRegistrar interface {
	Register(r Routes)
}

// NewRouter builds the engine and lets each injected handler mount its own
// routes. It constructs no handler itself.
func NewRouter(tokens usecase.TokenVerifier, corsOrigin string, log *slog.Logger, handlers ...RouteRegistrar) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery(), requestLog(log), cors(corsOrigin))

	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	v1 := r.Group("/api/v1")
	authed := v1.Group("", authenticate(tokens))
	routes := Routes{Public: v1, Authed: authed, Tenanted: authed.Group("", requireTenant())}
	for _, h := range handlers {
		h.Register(routes)
	}
	return r
}
