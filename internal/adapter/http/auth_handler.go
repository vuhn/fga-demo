package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *handlers) login(c *gin.Context) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !bind(c, &body) {
		return
	}
	res, err := h.app.Login(c, body.Email, body.Password)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"access_token": res.Token, "expires_at": res.ExpiresAt, "user": userView(res.User)})
}

func (h *handlers) me(c *gin.Context) {
	u, tenants, err := h.app.Me(c, actor(c))
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": userView(u), "tenants": tenantViews(tenants)})
}

// myPermissions returns the caller's own permissions on the platform and,
// with ?tenant_id=, on that tenant. The UI maps them to menus and buttons.
func (h *handlers) myPermissions(c *gin.Context) {
	p, err := h.app.MyPermissions(c, actor(c), c.Query("tenant_id"))
	if err != nil {
		writeErr(c, err)
		return
	}
	body := gin.H{"platform": p.Platform, "tenant_id": nil, "tenant": nil}
	if p.TenantID != "" {
		body["tenant_id"], body["tenant"] = p.TenantID, p.Tenant
	}
	c.JSON(http.StatusOK, body)
}
