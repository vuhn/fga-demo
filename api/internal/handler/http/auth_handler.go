package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/vuhn/fga-demo/api/internal/usecase"
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
	res, err := h.authenticator.Login(c, body.Email, body.Password)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"access_token": res.Token, "expires_at": res.ExpiresAt, "user": userView(res.User)})
}

func (h *AuthHandler) me(c *gin.Context) {
	u, tenants, err := h.profile.Me(c, actor(c))
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": userView(u), "tenants": tenantViews(tenants)})
}
