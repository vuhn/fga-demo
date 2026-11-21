package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/vuhn/fga-demo/api/internal/handler/http/mapper"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

// TaskUsecases is everything the task handler calls.
type TaskUsecases struct {
	Creator usecase.TaskCreator
}

type TaskHandler struct{ uc TaskUsecases }

func NewTaskHandler(uc TaskUsecases) *TaskHandler { return &TaskHandler{uc: uc} }

func (h *TaskHandler) Register(r Routes) {
	r.Tenanted.POST("/projects/:id/tasks", h.create)
}

func (h *TaskHandler) create(c *gin.Context) {
	var body mapper.TaskCreateBody
	if !bind(c, &body) {
		return
	}
	in, err := mapper.TaskCreatorInput(actor(c), tenant(c), c.Param("id"), body)
	if err != nil {
		writeErr(c, err)
		return
	}
	t, err := h.uc.Creator.Create(c, in)
	respond(c, http.StatusCreated, mapper.TaskView(t.Visible), err)
}
