package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/vuhn/fga-demo/internal/domain"
	"github.com/vuhn/fga-demo/internal/usecase"
)

// content serves one product: folders+documents or albums+photos.
type content struct {
	app  *usecase.App
	kind domain.Kind
}

func (h content) mount(g *gin.RouterGroup) {
	cont, item := "/"+h.kind.Container+"s", "/"+h.kind.Item+"s"
	g.GET(cont, h.listContainers)
	g.POST(cont, h.createContainer)
	g.GET(cont+"/:id", h.getContainer)
	g.PATCH(cont+"/:id", h.renameContainer)
	g.DELETE(cont+"/:id", h.deleteContainer)
	g.PUT(cont+"/:id/visibility", h.setVisibility)
	g.GET(item, h.listItems)
	g.POST(item, h.createItem)
	g.GET(item+"/:id", h.getItem)
	g.PATCH(item+"/:id", h.renameItem)
	g.DELETE(item+"/:id", h.deleteItem)
	for _, typ := range []string{h.kind.Container, h.kind.Item} {
		g.GET("/"+typ+"s/:id/grants", h.listGrants(typ))
		g.GET("/"+typ+"s/:id/share-candidates", h.shareCandidates(typ))
		g.POST("/"+typ+"s/:id/grants", h.share(typ))
		g.DELETE("/"+typ+"s/:id/grants", h.revoke(typ))
	}
}

func (h content) listContainers(c *gin.Context) {
	cs, err := h.app.ListContainers(c, actor(c), tenant(c), h.kind)
	respond(c, http.StatusOK, mapSlice(cs, visibleContainerView), err)
}

func (h content) createContainer(c *gin.Context) {
	var body struct {
		Name      string  `json:"name"`
		ParentID  *string `json:"parent_id"`
		IsPrivate *bool   `json:"is_private"`
	}
	if !bind(c, &body) {
		return
	}
	if body.IsPrivate == nil {
		writeErr(c, domain.Invalid("is_private is required"))
		return
	}
	ct, err := h.app.CreateContainer(c, actor(c), tenant(c), h.kind, body.Name, body.ParentID, *body.IsPrivate)
	respond(c, http.StatusCreated, containerView(ct), err)
}

func (h content) getContainer(c *gin.Context) {
	ct, err := h.app.GetContainer(c, actor(c), tenant(c), h.kind, c.Param("id"))
	respond(c, http.StatusOK, visibleContainerView(ct), err)
}

func (h content) renameContainer(c *gin.Context) {
	var body struct{ Name string }
	if !bind(c, &body) {
		return
	}
	ct, err := h.app.RenameContainer(c, actor(c), tenant(c), h.kind, c.Param("id"), body.Name)
	respond(c, http.StatusOK, containerView(ct), err)
}

func (h content) setVisibility(c *gin.Context) {
	var body struct {
		IsPrivate *bool `json:"is_private"`
	}
	if !bind(c, &body) {
		return
	}
	if body.IsPrivate == nil {
		writeErr(c, domain.Invalid("is_private is required"))
		return
	}
	ct, err := h.app.SetVisibility(c, actor(c), tenant(c), h.kind, c.Param("id"), *body.IsPrivate)
	respond(c, http.StatusOK, containerView(ct), err)
}

func (h content) deleteContainer(c *gin.Context) {
	if err := h.app.DeleteContainer(c, actor(c), tenant(c), h.kind, c.Param("id")); err != nil {
		writeErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h content) itemJSON(it domain.Item) map[string]any { return itemView(h.kind, it) }

func (h content) visibleItemJSON(v usecase.Visible[domain.Item]) map[string]any {
	m := itemView(h.kind, v.Value)
	m["permissions"] = v.Perms
	return m
}

func (h content) listItems(c *gin.Context) {
	its, err := h.app.ListItems(c, actor(c), tenant(c), h.kind, c.Query(h.kind.ContainerRef+"_id"))
	respond(c, http.StatusOK, mapSlice(its, h.visibleItemJSON), err)
}

func (h content) createItem(c *gin.Context) {
	var body map[string]any
	if !bind(c, &body) {
		return
	}
	in := usecase.NewItem{}
	in.ContainerID, _ = body[h.kind.ContainerRef+"_id"].(string)
	in.Name, _ = body["name"].(string)
	in.MimeType, _ = body["mime_type"].(string)
	size, ok := body["byte_size"].(float64)
	if !ok || size != float64(int64(size)) {
		writeErr(c, domain.Invalid("byte_size must be an integer"))
		return
	}
	in.ByteSize = int64(size)
	it, err := h.app.CreateItem(c, actor(c), tenant(c), h.kind, in)
	respond(c, http.StatusCreated, h.itemJSON(it), err)
}

func (h content) getItem(c *gin.Context) {
	it, err := h.app.GetItem(c, actor(c), tenant(c), h.kind, c.Param("id"))
	respond(c, http.StatusOK, h.visibleItemJSON(it), err)
}

func (h content) renameItem(c *gin.Context) {
	var body struct{ Name string }
	if !bind(c, &body) {
		return
	}
	it, err := h.app.RenameItem(c, actor(c), tenant(c), h.kind, c.Param("id"), body.Name)
	respond(c, http.StatusOK, h.itemJSON(it), err)
}

func (h content) deleteItem(c *gin.Context) {
	if err := h.app.DeleteItem(c, actor(c), tenant(c), h.kind, c.Param("id")); err != nil {
		writeErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

type shareBody struct {
	Relation string `json:"relation"`
	Subject  string `json:"subject"`
	UserID   string `json:"user_id"`
}

func (b shareBody) input() usecase.ShareInput {
	return usecase.ShareInput{Relation: b.Relation, Subject: b.Subject, UserID: b.UserID}
}

func (h content) listGrants(typ string) gin.HandlerFunc {
	return func(c *gin.Context) {
		gs, err := h.app.ListGrants(c, actor(c), tenant(c), typ, c.Param("id"))
		respond(c, http.StatusOK, mapSlice(gs, grantView), err)
	}
}

func (h content) shareCandidates(typ string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ms, err := h.app.ShareCandidates(c, actor(c), tenant(c), typ, c.Param("id"))
		respond(c, http.StatusOK, mapSlice(ms, memberView), err)
	}
}

func (h content) share(typ string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body shareBody
		if !bind(c, &body) {
			return
		}
		g, err := h.app.Share(c, actor(c), tenant(c), typ, c.Param("id"), body.input())
		respond(c, http.StatusCreated, grantView(g), err)
	}
}

func (h content) revoke(typ string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body shareBody
		if !bind(c, &body) {
			return
		}
		if err := h.app.Revoke(c, actor(c), tenant(c), typ, c.Param("id"), body.input()); err != nil {
			writeErr(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}
