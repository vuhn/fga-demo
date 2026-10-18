// Package http is the Gin adapter. Handlers decode JSON, read the actor and
// tenant, call one usecase method, and map errors to the catalog.
package http

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/vuhn/fga-demo/internal/domain"
	"github.com/vuhn/fga-demo/internal/usecase"
)

type TokenVerifier interface {
	Verify(token string) (userID string, err error)
}

const (
	ctxActor  = "actor"
	ctxTenant = "tenant"
)

func requestLog(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Info("http", "method", c.Request.Method, "path", c.Request.URL.Path, "status", c.Writer.Status(), "dur", time.Since(start))
	}
}

func cors(origin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("Access-Control-Allow-Origin", origin)
		h.Set("Access-Control-Allow-Headers", "Authorization, X-Tenant-ID, Content-Type")
		h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		h.Set("Vary", "Origin")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func authenticate(v TokenVerifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
		if !ok || raw == "" {
			writeErr(c, domain.ErrUnauthenticated)
			c.Abort()
			return
		}
		sub, err := v.Verify(raw)
		if err != nil {
			writeErr(c, domain.ErrUnauthenticated)
			c.Abort()
			return
		}
		c.Set(ctxActor, sub)
		c.Next()
	}
}

// requireTenant parses X-Tenant-ID. It does not check membership; the
// usecase's Checks do.
func requireTenant() gin.HandlerFunc {
	return func(c *gin.Context) {
		t := c.GetHeader("X-Tenant-ID")
		if !usecase.ValidUUID(t) {
			writeErr(c, domain.Invalid("X-Tenant-ID header must be a tenant UUID"))
			c.Abort()
			return
		}
		c.Set(ctxTenant, t)
		c.Next()
	}
}

func actor(c *gin.Context) string  { return c.GetString(ctxActor) }
func tenant(c *gin.Context) string { return c.GetString(ctxTenant) }

var errorCatalog = []struct {
	err    error
	status int
}{
	{domain.ErrAffiliationRoleMismatch, http.StatusBadRequest},
	{domain.ErrNotAMember, http.StatusBadRequest},
	{domain.ErrCannotUpload, http.StatusBadRequest},
	{domain.ErrValidation, http.StatusBadRequest},
	{domain.ErrUnauthenticated, http.StatusUnauthorized},
	{domain.ErrForbidden, http.StatusForbidden},
	{domain.ErrNotFound, http.StatusNotFound},
	{domain.ErrConflict, http.StatusConflict},
	{domain.ErrTupleWriteFailed, http.StatusInternalServerError},
}

func writeErr(c *gin.Context, err error) {
	status, code := http.StatusInternalServerError, "internal"
	for _, e := range errorCatalog {
		if errors.Is(err, e.err) {
			status, code = e.status, e.err.Error()
			break
		}
	}
	body := gin.H{"error": code}
	var ce *domain.CheckError
	if errors.As(err, &ce) {
		body["check"] = gin.H{"relation": ce.Relation, "object": ce.Object}
	}
	if status >= 500 {
		slog.Error("request failed", "path", c.Request.URL.Path, "err", err)
	} else if ce == nil && err.Error() != code {
		body["message"] = err.Error()
	}
	c.JSON(status, body)
}

// bind decodes JSON and reports malformed bodies as validation errors.
func bind(c *gin.Context, dst any) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		writeErr(c, domain.Invalid("malformed JSON: %v", err))
		return false
	}
	return true
}

func respond(c *gin.Context, status int, v any, err error) {
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(status, v)
}
