package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

// recentEstimateMessages is how much of the chat the engine sees besides the stored state.
const recentEstimateMessages = 12

type estimateSessionMessenger struct {
	blueprints gateway.BlueprintGateway
	sessions   gateway.EstimateSessionGateway
	engine     gateway.EstimateEngine
	authz      usecase.Authorizer
	ids        gateway.IDGenerator
}

func NewEstimateSessionMessenger(blueprints gateway.BlueprintGateway, sessions gateway.EstimateSessionGateway,
	engine gateway.EstimateEngine, authz usecase.Authorizer, ids gateway.IDGenerator) usecase.EstimateSessionMessenger {
	return &estimateSessionMessenger{blueprints: blueprints, sessions: sessions, engine: engine, authz: authz, ids: ids}
}

// Send runs one turn. Order (fail closed, like every engine-backed write): validate, load the blueprint in the
// tenant, Check can_edit on it, load its session, check the turn cap, then call the engine, and only then write
// the two messages, the state and any estimate together, guarded by the session's epoch and seq (409 when
// another turn finished first or the estimate was restarted meanwhile). A cancelled request or an engine error writes nothing, so the session never holds half an
// answer.
func (s *estimateSessionMessenger) Send(ctx context.Context, actor, tenantID, blueprintID, message string, overrides map[string]float64, onEvent func(entity.EstimateEvent)) (usecase.EstimateSessionDetail, error) {
	message = strings.TrimSpace(message)
	if len([]rune(message)) > entity.MaxEstimateMessage {
		return usecase.EstimateSessionDetail{}, entity.Invalid("message must be at most %d characters", entity.MaxEstimateMessage)
	}
	if message == "" && len(overrides) == 0 {
		return usecase.EstimateSessionDetail{}, entity.Invalid("say something or enter a rate")
	}
	for code, v := range overrides {
		if code == "" || v <= 0 || v > 1e12 {
			return usecase.EstimateSessionDetail{}, entity.Invalid("rate for %q must be a positive number", code)
		}
	}
	if _, err := loadBlueprint(ctx, s.blueprints, tenantID, blueprintID); err != nil {
		return usecase.EstimateSessionDetail{}, err
	}
	if err := s.authz.Require(ctx, actor, "can_edit", entity.ObjectTypeBlueprint, blueprintID); err != nil {
		return usecase.EstimateSessionDetail{}, err
	}
	sess, err := s.sessions.Get(ctx, blueprintID)
	if err != nil {
		return usecase.EstimateSessionDetail{}, err
	}
	if sess.Seq >= entity.MaxEstimateTurns {
		return usecase.EstimateSessionDetail{}, entity.Invalid("this session reached its %d-turn limit; start a new estimate", entity.MaxEstimateTurns)
	}
	v, err := s.blueprints.GetVersion(ctx, sess.BlueprintID, sess.BlueprintVersion)
	if err != nil {
		return usecase.EstimateSessionDetail{}, err
	}
	msgs, err := s.sessions.Messages(ctx, sess.BlueprintID)
	if err != nil {
		return usecase.EstimateSessionDetail{}, err
	}
	recent := msgs
	if len(recent) > recentEstimateMessages {
		recent = recent[len(recent)-recentEstimateMessages:]
	}
	turn, err := s.engine.EstimateTurn(ctx, v.Model, sess.State, message, recent, overrides, onEvent)
	if err != nil {
		return usecase.EstimateSessionDetail{}, err
	}
	if err := ctx.Err(); err != nil { // the client went away at the very end: write nothing
		return usecase.EstimateSessionDetail{}, err
	}
	if strings.TrimSpace(turn.Reply) == "" {
		return usecase.EstimateSessionDetail{}, fmt.Errorf("%w: engine returned an empty reply", entity.ErrEngineUnavailable)
	}

	userText := message
	if userText == "" {
		userText = describeOverrides(overrides)
	} else if len(overrides) > 0 {
		userText += "\n" + describeOverrides(overrides)
	}
	status := turn.Status
	if status == "" {
		status = entity.EstimateStatusOf(turn.State)
	}
	switch status {
	case entity.EstimateInterviewing, entity.EstimateResearching, entity.EstimateReady, entity.EstimateFailed:
	default:
		return usecase.EstimateSessionDetail{}, fmt.Errorf("%w: engine returned status %q", entity.ErrEngineUnavailable, status)
	}
	user := entity.EstimateMessage{BlueprintID: sess.BlueprintID, Role: "user", Content: userText}
	assistant := entity.EstimateMessage{BlueprintID: sess.BlueprintID, Role: "assistant", Content: turn.Reply}
	var est *entity.BlueprintEstimate
	if len(turn.Estimate) > 0 && string(turn.Estimate) != "null" {
		low, base, high, err := estimateTotals(turn.Estimate)
		if err != nil {
			return usecase.EstimateSessionDetail{}, fmt.Errorf("%w: bad estimate from engine: %v", entity.ErrEngineUnavailable, err)
		}
		est = &entity.BlueprintEstimate{ID: s.ids.NewID(), BlueprintID: sess.BlueprintID, Version: sess.BlueprintVersion,
			TotalLow: low, TotalBase: base, TotalHigh: high, Result: turn.Estimate}
	}
	if err := s.sessions.AppendTurn(ctx, sess, turn.State, user, assistant, est); err != nil {
		return usecase.EstimateSessionDetail{}, err
	}
	return readEstimateSession(ctx, s.sessions, sess.BlueprintID)
}

func describeOverrides(o map[string]float64) string {
	codes := make([]string, 0, len(o))
	for c := range o {
		codes = append(codes, c)
	}
	sort.Strings(codes)
	parts := make([]string, 0, len(codes))
	for _, c := range codes {
		parts = append(parts, fmt.Sprintf("%s = %.0f", c, o[c]))
	}
	return "Rates entered: " + strings.Join(parts, ", ")
}
