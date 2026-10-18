package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type UserHandler struct {
	lister  usecase.UserLister
	creator usecase.UserCreator
	updater usecase.UserUpdater
}

func NewUserHandler(lister usecase.UserLister, creator usecase.UserCreator, updater usecase.UserUpdater) *UserHandler {
	return &UserHandler{lister: lister, creator: creator, updater: updater}
}

func (h *UserHandler) Register(r Routes) {
	r.Authed.GET("/users", h.list)
	r.Authed.POST("/users", h.create)
	r.Authed.PATCH("/users/:id", h.update)
}

func (h *UserHandler) list(c *gin.Context) {
	us, err := h.lister.List(c, actor(c))
	respond(c, http.StatusOK, mapSlice(us, userView), err)
}

func (h *UserHandler) create(c *gin.Context) {
	var body struct{ Email, Name, Password string }
	if !bind(c, &body) {
		return
	}
	u, err := h.creator.Create(c, actor(c), usecase.CreateUserInput{Email: body.Email, Name: body.Name, Password: body.Password})
	respond(c, http.StatusCreated, userView(u), err)
}

func (h *UserHandler) update(c *gin.Context) {
	var body struct{ Name string }
	if !pathUUID(c, "id") || !bind(c, &body) {
		return
	}
	u, err := h.updater.UpdateName(c, actor(c), c.Param("id"), body.Name)
	respond(c, http.StatusOK, userView(u), err)
}
