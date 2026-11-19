package http

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

// EstimateUsecases is everything the estimate handler calls.
type EstimateUsecases struct {
	SessionCreator usecase.EstimateSessionCreator
	Messenger      usecase.EstimateSessionMessenger
	SessionGetter  usecase.EstimateSessionGetter
	EstimateGetter usecase.EstimateGetter
	EstimateLister usecase.EstimateLister
}

type EstimateHandler struct{ uc EstimateUsecases }

func NewEstimateHandler(uc EstimateUsecases) *EstimateHandler { return &EstimateHandler{uc: uc} }

func (h *EstimateHandler) Register(r Routes) {
	g := r.Tenanted
	g.POST("/blueprints/:id/estimate-session", h.startSession)
	g.GET("/blueprints/:id/estimate-session", h.getSession)
	g.POST("/blueprints/:id/estimate-session/messages", h.send)
	g.GET("/blueprints/:id/estimates", h.listEstimates)
	g.GET("/blueprints/:id/estimates/:eid", h.getEstimate)
}

func (h *EstimateHandler) startSession(c *gin.Context) {
	d, err := h.uc.SessionCreator.Create(c, actor(c), tenant(c), c.Param("id"))
	respond(c, http.StatusCreated, estimateSessionView(d), err)
}

func (h *EstimateHandler) listEstimates(c *gin.Context) {
	es, err := h.uc.EstimateLister.List(c, actor(c), tenant(c), c.Param("id"))
	respond(c, http.StatusOK, mapSlice(es, estimateSummary), err)
}

func (h *EstimateHandler) getEstimate(c *gin.Context) {
	e, err := h.uc.EstimateGetter.Get(c, actor(c), tenant(c), c.Param("id"), c.Param("eid"))
	respond(c, http.StatusOK, estimateView(e), err)
}

func (h *EstimateHandler) getSession(c *gin.Context) {
	d, err := h.uc.SessionGetter.Get(c, actor(c), tenant(c), c.Param("id"))
	respond(c, http.StatusOK, estimateSessionView(d), err)
}

// send runs one agent turn as Server-Sent Events: progress, token, fact, question, rate and estimate events as
// the engine produces them, then "done" with the saved session (or "error"). Every refusal (validation, Check,
// turn cap, engine down) happens before the first event and is a plain JSON status; the stream opens only
// with the first event. If the browser goes away the request context is cancelled, the engine run stops and
// nothing is saved.
func (h *EstimateHandler) send(c *gin.Context) {
	var body struct {
		Message   string             `json:"message"`
		Overrides map[string]float64 `json:"rate_overrides"`
	}
	if !bind(c, &body) {
		return
	}
	started := false
	emit := func(event string, data []byte) {
		if !started {
			started = true
			c.Header("Content-Type", "text/event-stream")
			c.Header("Cache-Control", "no-cache")
			c.Header("X-Accel-Buffering", "no")
			c.Status(http.StatusOK)
		}
		if len(data) == 0 {
			data = []byte("null")
		}
		fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event, data)
		c.Writer.Flush()
	}
	d, err := h.uc.Messenger.Send(c.Request.Context(), actor(c), tenant(c), c.Param("id"), body.Message, body.Overrides,
		func(ev model.EstimateEvent) { emit(ev.Name, ev.Data) })
	if err != nil {
		if !started {
			writeErr(c, err)
			return
		}
		status, eb := errBody(c, err)
		eb["status"] = status
		data, _ := json.Marshal(eb)
		emit("error", data)
		return
	}
	data, _ := json.Marshal(estimateSessionView(d))
	emit("done", data)
}
