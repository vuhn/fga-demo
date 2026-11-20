package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/vuhn/fga-demo/api/internal/handler/http/mapper"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
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
	us, err := h.lister.List(c, input.UserLister{Actor: actor(c)})
	respond(c, http.StatusOK, mapSlice(us.Items, mapper.UserView), err)
}

func (h *UserHandler) create(c *gin.Context) {
	var body struct {
		Email, Name, Password string
		CompanyID             string `json:"company_id"`
	}
	if !bind(c, &body) {
		return
	}
	u, err := h.creator.Create(c, input.UserCreator{Actor: actor(c), Email: body.Email, Name: body.Name, Password: body.Password, CompanyID: body.CompanyID})
	respond(c, http.StatusCreated, mapper.UserView(u.User), err)
}

func (h *UserHandler) update(c *gin.Context) {
	var body struct{ Name string }
	if !pathUUID(c, "id") || !bind(c, &body) {
		return
	}
	u, err := h.updater.UpdateName(c, input.UserUpdater{Actor: actor(c), ID: c.Param("id"), Name: body.Name})
	respond(c, http.StatusOK, mapper.UserView(u.User), err)
}
