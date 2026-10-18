package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type ProductHandler struct {
	lister usecase.ProductLister
	setter usecase.ProductSetter
}

func NewProductHandler(lister usecase.ProductLister, setter usecase.ProductSetter) *ProductHandler {
	return &ProductHandler{lister: lister, setter: setter}
}

func (h *ProductHandler) Register(r Routes) {
	r.Authed.GET("/tenants/:id/products", h.list)
	r.Authed.PUT("/tenants/:id/products/:product", h.set)
}

func (h *ProductHandler) list(c *gin.Context) {
	if !pathUUID(c, "id") {
		return
	}
	ps, err := h.lister.List(c, actor(c), c.Param("id"))
	respond(c, http.StatusOK, productsView(ps), err)
}

func (h *ProductHandler) set(c *gin.Context) {
	var body struct {
		Enabled *bool `json:"enabled"`
	}
	if !pathUUID(c, "id") || !bind(c, &body) {
		return
	}
	if body.Enabled == nil {
		writeErr(c, entity.Invalid("enabled is required"))
		return
	}
	ps, err := h.setter.Set(c, actor(c), c.Param("id"), c.Param("product"), *body.Enabled)
	respond(c, http.StatusOK, productsView(ps), err)
}
