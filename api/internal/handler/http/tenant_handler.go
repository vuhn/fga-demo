package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type TenantHandler struct {
	lister  usecase.TenantLister
	mine    usecase.MyTenantLister
	creator usecase.TenantCreator
	getter  usecase.TenantGetter
	updater usecase.TenantUpdater
}

func NewTenantHandler(lister usecase.TenantLister, mine usecase.MyTenantLister, creator usecase.TenantCreator,
	getter usecase.TenantGetter, updater usecase.TenantUpdater) *TenantHandler {
	return &TenantHandler{lister: lister, mine: mine, creator: creator, getter: getter, updater: updater}
}

func (h *TenantHandler) Register(r Routes) {
	r.Authed.GET("/tenants", h.list)
	r.Authed.POST("/tenants", h.create)
	r.Authed.GET("/tenants/mine", h.listMine)
	r.Authed.GET("/tenants/:id", h.get)
	r.Authed.PATCH("/tenants/:id", h.update)
}

func (h *TenantHandler) list(c *gin.Context) {
	ts, err := h.lister.List(c, actor(c))
	respond(c, http.StatusOK, mapSlice(ts, tenantView), err)
}

func (h *TenantHandler) create(c *gin.Context) {
	var body struct{ Name, Slug string }
	if !bind(c, &body) {
		return
	}
	t, err := h.creator.Create(c, actor(c), body.Name, body.Slug)
	respond(c, http.StatusCreated, tenantView(t), err)
}

func (h *TenantHandler) listMine(c *gin.Context) {
	ts, err := h.mine.ListMine(c, actor(c))
	respond(c, http.StatusOK, tenantViews(ts), err)
}

func (h *TenantHandler) get(c *gin.Context) {
	if !pathUUID(c, "id") {
		return
	}
	t, err := h.getter.Get(c, actor(c), c.Param("id"))
	respond(c, http.StatusOK, tenantView(t), err)
}

func (h *TenantHandler) update(c *gin.Context) {
	var body struct{ Name string }
	if !pathUUID(c, "id") || !bind(c, &body) {
		return
	}
	t, err := h.updater.UpdateName(c, actor(c), c.Param("id"), body.Name)
	respond(c, http.StatusOK, tenantView(t), err)
}
