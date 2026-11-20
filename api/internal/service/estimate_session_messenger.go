package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

// recentEstimateMessages is how much of the chat the engine sees besides the stored state.
const recentEstimateMessages = 12

type estimateSessionMessenger struct {
	blueprints gateway.BlueprintGateway
	sessions   gateway.EstimateSessionGateway
	agent      gateway.EstimateAgent
	authz      helper.Authorizer
	ids        gateway.IDGenerator
}

func NewEstimateSessionMessenger(blueprints gateway.BlueprintGateway, sessions gateway.EstimateSessionGateway,
	agent gateway.EstimateAgent, authz helper.Authorizer, ids gateway.IDGenerator) usecase.EstimateSessionMessenger {
	return &estimateSessionMessenger{blueprints: blueprints, sessions: sessions, agent: agent, authz: authz, ids: ids}
}

// Send runs one turn. Order (fail closed, like every engine-backed write): validate, load the blueprint in the
// tenant, Check can_edit on it, load its session, check the turn cap, then call the engine, and only then write
// the two messages, the state and any estimate together, guarded by the session's epoch and seq (409 when
// another turn finished first or the estimate was restarted meanwhile). A cancelled request or an engine error writes nothing, so the session never holds half an
// answer.
func (s *estimateSessionMessenger) Send(ctx context.Context, in input.EstimateSessionMessenger, onEvent func(model.EstimateEvent)) (output.EstimateSessionMessenger, error) {
	in.Message = strings.TrimSpace(in.Message)
	if len([]rune(in.Message)) > model.MaxEstimateMessage {
		return output.EstimateSessionMessenger{}, model.Invalid("message must be at most %d characters", model.MaxEstimateMessage)
	}
	if in.Message == "" && len(in.Overrides) == 0 {
		return output.EstimateSessionMessenger{}, model.Invalid("say something or enter a rate")
	}
	for code, v := range in.Overrides {
		if code == "" || v <= 0 || v > 1e12 {
			return output.EstimateSessionMessenger{}, model.Invalid("rate for %q must be a positive number", code)
		}
	}
	if _, err := loadBlueprint(ctx, s.blueprints, in.TenantID, in.BlueprintID); err != nil {
		return output.EstimateSessionMessenger{}, err
	}
	if err := s.authz.Require(ctx, in.Actor, "can_edit", model.ObjectTypeBlueprint, in.BlueprintID); err != nil {
		return output.EstimateSessionMessenger{}, err
	}
	sess, err := s.sessions.Get(ctx, in.BlueprintID)
	if err != nil {
		return output.EstimateSessionMessenger{}, err
	}
	if sess.Seq >= model.MaxEstimateTurns {
		return output.EstimateSessionMessenger{}, model.Invalid("this session reached its %d-turn limit; start a new estimate", model.MaxEstimateTurns)
	}
	v, err := s.blueprints.GetVersion(ctx, sess.BlueprintID, sess.BlueprintVersion)
	if err != nil {
		return output.EstimateSessionMessenger{}, err
	}
	msgs, err := s.sessions.Messages(ctx, sess.BlueprintID)
	if err != nil {
		return output.EstimateSessionMessenger{}, err
	}
	recent := msgs
	if len(recent) > recentEstimateMessages {
		recent = recent[len(recent)-recentEstimateMessages:]
	}
	turn, err := s.agent.EstimateTurn(ctx, model.EstimateTurnParams{Model: v.Model, State: sess.State, Message: in.Message, Messages: recent, Overrides: in.Overrides}, onEvent)
	if err != nil {
		return output.EstimateSessionMessenger{}, err
	}
	if err := ctx.Err(); err != nil { // the client went away at the very end: write nothing
		return output.EstimateSessionMessenger{}, err
	}
	if strings.TrimSpace(turn.Reply) == "" {
		return output.EstimateSessionMessenger{}, fmt.Errorf("%w: engine returned an empty reply", model.ErrEngineUnavailable)
	}

	userText := in.Message
	if userText == "" {
		userText = describeOverrides(in.Overrides)
	} else if len(in.Overrides) > 0 {
		userText += "\n" + describeOverrides(in.Overrides)
	}
	status := turn.Status
	if status == "" {
		status = model.EstimateStatusOf(turn.State)
	}
	switch status {
	case model.EstimateInterviewing, model.EstimateResearching, model.EstimateReady, model.EstimateFailed:
	default:
		return output.EstimateSessionMessenger{}, fmt.Errorf("%w: engine returned status %q", model.ErrEngineUnavailable, status)
	}
	user := model.EstimateMessage{BlueprintID: sess.BlueprintID, Role: "user", Content: userText}
	assistant := model.EstimateMessage{BlueprintID: sess.BlueprintID, Role: "assistant", Content: turn.Reply}
	var est *model.BlueprintEstimate
	if len(turn.Estimate) > 0 && string(turn.Estimate) != "null" {
		low, base, high, err := estimateTotals(turn.Estimate)
		if err != nil {
			return output.EstimateSessionMessenger{}, fmt.Errorf("%w: bad estimate from engine: %v", model.ErrEngineUnavailable, err)
		}
		est = &model.BlueprintEstimate{ID: s.ids.NewID(), BlueprintID: sess.BlueprintID, Version: sess.BlueprintVersion,
			TotalLow: low, TotalBase: base, TotalHigh: high, Result: turn.Estimate}
	}
	if err := s.sessions.AppendTurn(ctx, model.AppendEstimateTurnParams{Session: sess, State: turn.State, User: user, Assistant: assistant, Estimate: est}); err != nil {
		return output.EstimateSessionMessenger{}, err
	}
	got, err := readEstimateSession(ctx, s.sessions, sess.BlueprintID)
	return output.EstimateSessionMessenger{EstimateSessionDetail: estimateSessionDetailOutput(got)}, err
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
