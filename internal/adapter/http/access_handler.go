package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/vuhn/fga-demo/internal/usecase"
)

type checkBody struct {
	UserID     string `json:"user_id"`
	Relation   string `json:"relation"`
	ObjectType string `json:"object_type"`
	ObjectID   string `json:"object_id"`
}

func (b checkBody) req() usecase.CheckRequest {
	return usecase.CheckRequest{UserID: b.UserID, Relation: b.Relation, ObjectType: b.ObjectType, ObjectID: b.ObjectID}
}

func (h *handlers) check(c *gin.Context) {
	var body checkBody
	if !bind(c, &body) {
		return
	}
	ok, err := h.app.Inspect(c, actor(c), body.req())
	respond(c, http.StatusOK, gin.H{"allowed": ok}, err)
}

func (h *handlers) batchCheck(c *gin.Context) {
	var body struct {
		Checks []checkBody `json:"checks"`
	}
	if !bind(c, &body) {
		return
	}
	res, err := h.app.BatchInspect(c, actor(c), mapSlice(body.Checks, checkBody.req))
	results := mapSlice(res, func(b bool) gin.H { return gin.H{"allowed": b} })
	respond(c, http.StatusOK, gin.H{"results": results}, err)
}

func (h *handlers) access(c *gin.Context) {
	r, err := h.app.Access(c, actor(c), c.Query("object_type"), c.Query("object_id"))
	if err != nil {
		writeErr(c, err)
		return
	}
	members := mapSlice(r.Members, func(m usecase.MemberAccess) gin.H {
		return gin.H{"user_id": m.UserID, "name": m.Name, "can_view": m.CanView, "can_edit": m.CanEdit, "can_delete": m.CanDelete}
	})
	tuples := mapSlice(r.DirectTuples, func(t usecase.Tuple) tupleJSON { return tupleJSON{t.User, t.Relation, t.Object} })
	c.JSON(http.StatusOK, gin.H{"object": r.Object, "direct_tuples": tuples, "members": members})
}
