package http

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/usecase"
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

type companyJSON struct {
	ID                string     `json:"id"`
	Name              string     `json:"name"`
	TaxID             string     `json:"tax_id,omitempty"`
	CreatedByTenantID string     `json:"created_by_tenant_id,omitempty"`
	TenantID          string     `json:"tenant_id,omitempty"`
	Claimed           bool       `json:"claimed"`
	ClaimedAt         *time.Time `json:"claimed_at"`
	MemberCount       int        `json:"member_count"`
	CreatedAt         time.Time  `json:"created_at"`
}

func companyView(c entity.Company) companyJSON {
	return companyJSON{c.ID, c.Name, c.TaxID, c.CreatedByTenantID, c.TenantID, c.Claimed(), c.ClaimedAt, c.MemberCount, c.CreatedAt}
}

func (h *CompanyHandler) listAll(c *gin.Context) {
	cs, err := h.allLister.ListAll(c, actor(c))
	respond(c, http.StatusOK, mapSlice(cs, companyView), err)
}

func (h *CompanyHandler) list(c *gin.Context) {
	if !pathUUID(c, "id") {
		return
	}
	cs, err := h.lister.List(c, actor(c), c.Param("id"))
	respond(c, http.StatusOK, mapSlice(cs, companyView), err)
}

func (h *CompanyHandler) create(c *gin.Context) {
	var body struct {
		Name  string `json:"name"`
		TaxID string `json:"tax_id"`
	}
	if !pathUUID(c, "id") || !bind(c, &body) {
		return
	}
	co, err := h.creator.Create(c, actor(c), c.Param("id"), usecase.CreateCompanyInput{Name: body.Name, TaxID: body.TaxID})
	respond(c, http.StatusCreated, companyView(co), err)
}

func (h *CompanyHandler) claim(c *gin.Context) {
	var body struct {
		CompanyIDs []string `json:"company_ids"`
	}
	if !pathUUID(c, "id") || !bind(c, &body) {
		return
	}
	co, err := h.claimer.Claim(c, actor(c), c.Param("id"), body.CompanyIDs)
	respond(c, http.StatusOK, companyView(co), err)
}
