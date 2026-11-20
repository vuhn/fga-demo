package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/vuhn/fga-demo/api/internal/handler/http/mapper"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
)

type AuthHandler struct {
	authenticator usecase.Authenticator
	profile       usecase.ProfileGetter
}

func NewAuthHandler(authenticator usecase.Authenticator, profile usecase.ProfileGetter) *AuthHandler {
	return &AuthHandler{authenticator: authenticator, profile: profile}
}

func (h *AuthHandler) Register(r Routes) {
	r.Public.POST("/auth/login", h.login)
	r.Authed.GET("/me", h.me)
}

func (h *AuthHandler) login(c *gin.Context) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !bind(c, &body) {
		return
	}
	res, err := h.authenticator.Login(c, input.Authenticator{Email: body.Email, Password: body.Password})
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"access_token": res.Token, "expires_at": res.ExpiresAt, "user": mapper.UserView(res.User)})
}

func (h *AuthHandler) me(c *gin.Context) {
	res, err := h.profile.Me(c, input.ProfileGetter{Actor: actor(c)})
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": mapper.UserView(res.User), "tenants": mapper.TenantViews(res.Tenants)})
}
