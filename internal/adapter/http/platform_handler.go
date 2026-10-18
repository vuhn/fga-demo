package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/vuhn/fga-demo/internal/domain"
	"github.com/vuhn/fga-demo/internal/usecase"
)

func (h *handlers) listUsers(c *gin.Context) {
	us, err := h.app.ListUsers(c, actor(c))
	respond(c, http.StatusOK, mapSlice(us, userView), err)
}

func (h *handlers) createUser(c *gin.Context) {
	var body struct{ Email, Name, Password string }
	if !bind(c, &body) {
		return
	}
	u, err := h.app.CreateUser(c, actor(c), body.Email, body.Name, body.Password)
	respond(c, http.StatusCreated, userView(u), err)
}

func (h *handlers) updateUser(c *gin.Context) {
	var body struct{ Name string }
	if !pathUUID(c, "id") || !bind(c, &body) {
		return
	}
	u, err := h.app.UpdateUser(c, actor(c), c.Param("id"), body.Name)
	respond(c, http.StatusOK, userView(u), err)
}

func (h *handlers) listTenants(c *gin.Context) {
	ts, err := h.app.ListTenants(c, actor(c))
	respond(c, http.StatusOK, mapSlice(ts, tenantView), err)
}

func (h *handlers) createTenant(c *gin.Context) {
	var body struct{ Name, Slug string }
	if !bind(c, &body) {
		return
	}
	t, err := h.app.CreateTenant(c, actor(c), body.Name, body.Slug)
	respond(c, http.StatusCreated, tenantView(t), err)
}

func (h *handlers) myTenants(c *gin.Context) {
	ts, err := h.app.MyTenants(c, actor(c))
	respond(c, http.StatusOK, tenantViews(ts), err)
}

func (h *handlers) getTenant(c *gin.Context) {
	if !pathUUID(c, "id") {
		return
	}
	t, err := h.app.GetTenant(c, actor(c), c.Param("id"))
	respond(c, http.StatusOK, tenantView(t), err)
}

func (h *handlers) updateTenant(c *gin.Context) {
	var body struct{ Name string }
	if !pathUUID(c, "id") || !bind(c, &body) {
		return
	}
	t, err := h.app.UpdateTenant(c, actor(c), c.Param("id"), body.Name)
	respond(c, http.StatusOK, tenantView(t), err)
}

func (h *handlers) listProducts(c *gin.Context) {
	if !pathUUID(c, "id") {
		return
	}
	ps, err := h.app.ListProducts(c, actor(c), c.Param("id"))
	respond(c, http.StatusOK, productsView(ps), err)
}

func (h *handlers) setProduct(c *gin.Context) {
	var body struct {
		Enabled *bool `json:"enabled"`
	}
	if !pathUUID(c, "id") || !bind(c, &body) {
		return
	}
	if body.Enabled == nil {
		writeErr(c, domain.Invalid("enabled is required"))
		return
	}
	ps, err := h.app.SetProduct(c, actor(c), c.Param("id"), c.Param("product"), *body.Enabled)
	respond(c, http.StatusOK, productsView(ps), err)
}

func pathUUID(c *gin.Context, name string) bool {
	if !usecase.ValidUUID(c.Param(name)) {
		writeErr(c, domain.Invalid("%s must be a UUID", name))
		return false
	}
	return true
}
