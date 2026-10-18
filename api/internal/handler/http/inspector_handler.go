package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type InspectorHandler struct {
	inspector usecase.Inspector
}

func NewInspectorHandler(inspector usecase.Inspector) *InspectorHandler {
	return &InspectorHandler{inspector: inspector}
}

func (h *InspectorHandler) Register(r Routes) {
	r.Authed.POST("/authz/check", h.check)
	r.Authed.POST("/authz/batch-check", h.batchCheck)
}

type checkBody struct {
	UserID     string `json:"user_id"`
	Relation   string `json:"relation"`
	ObjectType string `json:"object_type"`
	ObjectID   string `json:"object_id"`
}

func (b checkBody) req() entity.CheckRequest {
	return entity.CheckRequest{UserID: b.UserID, Relation: b.Relation, ObjectType: b.ObjectType, ObjectID: b.ObjectID}
}

func (h *InspectorHandler) check(c *gin.Context) {
	var body checkBody
	if !bind(c, &body) {
		return
	}
	ok, err := h.inspector.Inspect(c, actor(c), body.req())
	respond(c, http.StatusOK, gin.H{"allowed": ok}, err)
}

func (h *InspectorHandler) batchCheck(c *gin.Context) {
	var body struct {
		Checks []checkBody `json:"checks"`
	}
	if !bind(c, &body) {
		return
	}
	res, err := h.inspector.BatchInspect(c, actor(c), mapSlice(body.Checks, checkBody.req))
	results := mapSlice(res, func(b bool) gin.H { return gin.H{"allowed": b} })
	respond(c, http.StatusOK, gin.H{"results": results}, err)
}
