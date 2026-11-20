package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/vuhn/fga-demo/api/internal/handler/http/mapper"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
)

type CompanyHandler struct {
	lister    usecase.CompanyLister
	allLister usecase.AllCompanyLister
	creator   usecase.CompanyCreator
	claimer   usecase.CompanyClaimer
}

func NewCompanyHandler(lister usecase.CompanyLister, allLister usecase.AllCompanyLister, creator usecase.CompanyCreator, claimer usecase.CompanyClaimer) *CompanyHandler {
	return &CompanyHandler{lister: lister, allLister: allLister, creator: creator, claimer: claimer}
}

func (h *CompanyHandler) Register(r Routes) {
	r.Authed.GET("/companies", h.listAll)
	r.Authed.GET("/tenants/:id/companies", h.list)
	r.Authed.POST("/tenants/:id/companies", h.create)
	r.Authed.POST("/tenants/:id/company-claims", h.claim)
}

func (h *CompanyHandler) listAll(c *gin.Context) {
	cs, err := h.allLister.ListAll(c, input.AllCompanyLister{Actor: actor(c)})
	respond(c, http.StatusOK, mapSlice(cs.Items, mapper.CompanyView), err)
}

func (h *CompanyHandler) list(c *gin.Context) {
	if !pathUUID(c, "id") {
		return
	}
	cs, err := h.lister.List(c, input.CompanyLister{Actor: actor(c), TenantID: c.Param("id")})
	respond(c, http.StatusOK, mapSlice(cs.Items, mapper.CompanyView), err)
}

func (h *CompanyHandler) create(c *gin.Context) {
	var body struct {
		Name  string `json:"name"`
		TaxID string `json:"tax_id"`
	}
	if !pathUUID(c, "id") || !bind(c, &body) {
		return
	}
	co, err := h.creator.Create(c, input.CompanyCreator{Actor: actor(c), TenantID: c.Param("id"), Name: body.Name, TaxID: body.TaxID})
	respond(c, http.StatusCreated, mapper.CompanyView(co.Company), err)
}

func (h *CompanyHandler) claim(c *gin.Context) {
	var body struct {
		CompanyIDs []string `json:"company_ids"`
	}
	if !pathUUID(c, "id") || !bind(c, &body) {
		return
	}
	co, err := h.claimer.Claim(c, input.CompanyClaimer{Actor: actor(c), TenantID: c.Param("id"), CompanyIDs: body.CompanyIDs})
	respond(c, http.StatusOK, mapper.CompanyView(co.Company), err)
}
