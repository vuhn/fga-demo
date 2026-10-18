package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type AccessHandler struct {
	reporter usecase.AccessReporter
}

func NewAccessHandler(reporter usecase.AccessReporter) *AccessHandler {
	return &AccessHandler{reporter: reporter}
}

func (h *AccessHandler) Register(r Routes) {
	r.Authed.GET("/access", h.report)
}

func (h *AccessHandler) report(c *gin.Context) {
	rep, err := h.reporter.Report(c, actor(c), c.Query("object_type"), c.Query("object_id"))
	if err != nil {
		writeErr(c, err)
		return
	}
	members := mapSlice(rep.Members, func(m usecase.MemberAccess) gin.H {
		return gin.H{"user_id": m.UserID, "name": m.Name, "can_view": m.CanView, "can_edit": m.CanEdit, "can_delete": m.CanDelete}
	})
	tuples := mapSlice(rep.DirectTuples, func(t entity.Tuple) tupleJSON { return tupleJSON{t.User, t.Relation, t.Object} })
	c.JSON(http.StatusOK, gin.H{"object": rep.Object, "direct_tuples": tuples, "members": members})
}
