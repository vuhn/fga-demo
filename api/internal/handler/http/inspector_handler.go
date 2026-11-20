package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
)

type InspectorHandler struct {
	inspector usecase.Inspector
	batch     usecase.BatchInspector
}

func NewInspectorHandler(inspector usecase.Inspector, batch usecase.BatchInspector) *InspectorHandler {
	return &InspectorHandler{inspector: inspector, batch: batch}
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

func (b checkBody) req() input.Check {
	return input.Check{UserID: b.UserID, Relation: b.Relation, ObjectType: b.ObjectType, ObjectID: b.ObjectID}
}

func (h *InspectorHandler) check(c *gin.Context) {
	var body checkBody
	if !bind(c, &body) {
		return
	}
	ck := body.req()
	res, err := h.inspector.Inspect(c, input.Inspector{Actor: actor(c), UserID: ck.UserID, Relation: ck.Relation, ObjectType: ck.ObjectType, ObjectID: ck.ObjectID})
	respond(c, http.StatusOK, gin.H{"allowed": res.Allowed}, err)
}

func (h *InspectorHandler) batchCheck(c *gin.Context) {
	var body struct {
		Checks []checkBody `json:"checks"`
	}
	if !bind(c, &body) {
		return
	}
	res, err := h.batch.BatchInspect(c, input.BatchInspector{Actor: actor(c), Checks: mapSlice(body.Checks, checkBody.req)})
	results := mapSlice(res.Allowed, func(b bool) gin.H { return gin.H{"allowed": b} })
	respond(c, http.StatusOK, gin.H{"results": results}, err)
}
