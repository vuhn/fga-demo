package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/vuhn/fga-demo/api/internal/handler/http/mapper"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
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
	rep, err := h.reporter.Report(c, input.AccessReporter{Actor: actor(c), ObjectType: c.Query("object_type"), ObjectID: c.Query("object_id")})
	if err != nil {
		writeErr(c, err)
		return
	}
	members := mapSlice(rep.Members, func(m output.MemberAccess) gin.H {
		return gin.H{"user_id": m.UserID, "name": m.Name, "can_view": m.CanView, "can_edit": m.CanEdit, "can_delete": m.CanDelete}
	})
	tuples := mapSlice(rep.DirectTuples, mapper.TupleView)
	c.JSON(http.StatusOK, gin.H{"object": rep.Object, "direct_tuples": tuples, "members": members})
}
