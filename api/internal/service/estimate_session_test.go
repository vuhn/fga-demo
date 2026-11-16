package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
)

type fakeSessions struct {
	gateway.EstimateSessionGateway
	sess       entity.EstimateSession
	msgs       []entity.EstimateMessage
	appended   []entity.EstimateMessage
	est        *entity.BlueprintEstimate
	conflict   bool
	writes     int
	replaced   []entity.EstimateSession
	replaceErr error
}

// Replace mirrors the repo: the row is reset, its epoch raised and its messages deleted.
func (f *fakeSessions) Replace(_ context.Context, s entity.EstimateSession) error {
	if f.replaceErr != nil {
		return f.replaceErr
	}
	f.replaced = append(f.replaced, s)
	s.Epoch = f.sess.Epoch + 1
	f.sess, f.msgs, f.appended = s, nil, nil
	return nil
}

func (f *fakeSessions) Get(context.Context, string) (entity.EstimateSession, error) {
	if f.sess.BlueprintID == "" {
		return entity.EstimateSession{}, entity.ErrNotFound
	}
	return f.sess, nil
}
func (f *fakeSessions) Messages(context.Context, string) ([]entity.EstimateMessage, error) {
	return append(f.msgs, f.appended...), nil
}
func (f *fakeSessions) AppendTurn(_ context.Context, s entity.EstimateSession, state []byte, u, a entity.EstimateMessage, e *entity.BlueprintEstimate) error {
	if f.conflict || s.Seq != f.sess.Seq || s.Epoch != f.sess.Epoch {
		return entity.Conflict("another turn finished first")
	}
	f.writes++
	f.sess.State, f.sess.Seq = state, f.sess.Seq+1
	f.appended, f.est = append(f.appended, u, a), e
	return nil
}

type fakeEstimateEngine struct {
	calls  int
	during func() // runs while the engine "thinks" (a restart in another tab)
	turn   entity.EstimateTurn
	err    error
	got    struct {
		message   string
		overrides map[string]float64
		history   int
	}
}

func (e *fakeEstimateEngine) EstimateTurn(_ context.Context, _, _ []byte, message string, msgs []entity.EstimateMessage, o map[string]float64, on func(entity.EstimateEvent)) (entity.EstimateTurn, error) {
	e.calls++
	if e.during != nil {
		e.during()
	}
	e.got.message, e.got.overrides, e.got.history = message, o, len(msgs)
	on(entity.EstimateEvent{Name: "token", Data: json.RawMessage(`{"text":"hi"}`)})
	return e.turn, e.err
}

type seqIDs struct{ n int }

func (s *seqIDs) NewID() string {
	s.n++
	return "33333333-3333-3333-3333-33333333333" + string(rune('0'+s.n))
}

func messenger(sess *fakeSessions, eng *fakeEstimateEngine, deny bool) (func(msg string, o map[string]float64) (string, error), *[]string) {
	events := []string{}
	m := NewEstimateSessionMessenger(&fakeBlueprints{brief: "null"}, sess, eng, &fakeAuthz{deny: deny}, &seqIDs{})
	return func(msg string, o map[string]float64) (string, error) {
		_, err := m.Send(context.Background(), "u", "t", bpID, msg, o, func(ev entity.EstimateEvent) { events = append(events, ev.Name) })
		return strings.Join(events, ","), err
	}, &events
}

func newSess() *fakeSessions {
	return &fakeSessions{sess: entity.EstimateSession{BlueprintID: bpID, BlueprintVersion: 1, Epoch: 1, State: json.RawMessage(`{}`)}}
}

func okTurn() entity.EstimateTurn {
	return entity.EstimateTurn{State: json.RawMessage(`{"status":"ready"}`), Reply: "The total is ready.",
		Estimate: json.RawMessage(`{"total":{"low":900,"base":1000,"high":1300}}`)}
}

func TestEstimateTurnStoresBothMessagesStateAndEstimateTogether(t *testing.T) {
	sess, eng := newSess(), &fakeEstimateEngine{turn: okTurn()}
	send, _ := messenger(sess, eng, false)
	events, err := send("  Ho Chi Minh City  ", nil)
	if err != nil || events != "token" {
		t.Fatalf("err %v events %q", err, events)
	}
	if eng.got.message != "Ho Chi Minh City" {
		t.Fatalf("message not trimmed: %q", eng.got.message)
	}
	if sess.writes != 1 || len(sess.appended) != 2 || sess.appended[0].Role != "user" || sess.appended[1].Content != "The total is ready." {
		t.Fatalf("stored %+v", sess.appended)
	}
	if sess.sess.Status() != "ready" || sess.sess.Seq != 1 {
		t.Fatalf("session %+v", sess.sess)
	}
	if sess.est == nil || sess.est.TotalLow != 900 || sess.est.TotalBase != 1000 || sess.est.TotalHigh != 1300 || sess.est.Version != 1 {
		t.Fatalf("estimate %+v", sess.est)
	}
}

func TestEstimateTurnDeniedOrInvalidNeverReachesTheEngine(t *testing.T) {
	for name, tc := range map[string]struct {
		deny bool
		msg  string
		o    map[string]float64
		want error
	}{
		"viewer":      {deny: true, msg: "hi", want: entity.ErrForbidden},
		"empty":       {msg: "  ", want: entity.ErrValidation},
		"too long":    {msg: strings.Repeat("x", entity.MaxEstimateMessage+1), want: entity.ErrValidation},
		"bad rate":    {msg: "", o: map[string]float64{"paint": -1}, want: entity.ErrValidation},
		"rate no key": {msg: "", o: map[string]float64{"": 5}, want: entity.ErrValidation},
	} {
		sess, eng := newSess(), &fakeEstimateEngine{turn: okTurn()}
		send, _ := messenger(sess, eng, tc.deny)
		if _, err := send(tc.msg, tc.o); !errors.Is(err, tc.want) {
			t.Errorf("%s: want %v, got %v", name, tc.want, err)
		}
		if eng.calls != 0 || sess.writes != 0 {
			t.Errorf("%s: engine %d writes %d", name, eng.calls, sess.writes)
		}
	}
}

func TestEstimateTurnInAnotherTenantIsNotFound(t *testing.T) {
	sess, eng := newSess(), &fakeEstimateEngine{turn: okTurn()}
	m := NewEstimateSessionMessenger(&fakeBlueprints{brief: "null"}, sess, eng, &fakeAuthz{}, &seqIDs{})
	if _, err := m.Send(context.Background(), "u", "other", bpID, "hi", nil, func(entity.EstimateEvent) {}); !errors.Is(err, entity.ErrNotFound) || eng.calls != 0 {
		t.Fatalf("err %v calls %d", err, eng.calls)
	}
}

func TestEstimateTurnTurnCapStopsBeforeTheEngine(t *testing.T) {
	sess, eng := newSess(), &fakeEstimateEngine{turn: okTurn()}
	sess.sess.Seq = entity.MaxEstimateTurns
	send, _ := messenger(sess, eng, false)
	if _, err := send("hi", nil); !errors.Is(err, entity.ErrValidation) || eng.calls != 0 {
		t.Fatalf("err %v calls %d", err, eng.calls)
	}
}

func TestEstimateTurnEngineErrorOrAbortWritesNothing(t *testing.T) {
	sess, eng := newSess(), &fakeEstimateEngine{err: entity.ErrEngineUnavailable}
	send, _ := messenger(sess, eng, false)
	if _, err := send("hi", nil); !errors.Is(err, entity.ErrEngineUnavailable) || sess.writes != 0 {
		t.Fatalf("err %v writes %d", err, sess.writes)
	}
	// the browser went away after the engine finished
	ctx, cancel := context.WithCancel(context.Background())
	eng2 := &fakeEstimateEngine{turn: okTurn()}
	m := NewEstimateSessionMessenger(&fakeBlueprints{brief: "null"}, sess, eng2, &fakeAuthz{}, &seqIDs{})
	cancel()
	if _, err := m.Send(ctx, "u", "t", bpID, "hi", nil, func(entity.EstimateEvent) {}); !errors.Is(err, context.Canceled) || sess.writes != 0 {
		t.Fatalf("err %v writes %d", err, sess.writes)
	}
}

func TestEstimateTurnConcurrentTurnIs409AndWritesNothing(t *testing.T) {
	sess, eng := newSess(), &fakeEstimateEngine{turn: okTurn()}
	sess.conflict = true
	send, _ := messenger(sess, eng, false)
	if _, err := send("hi", nil); !errors.Is(err, entity.ErrConflict) || sess.writes != 0 {
		t.Fatalf("err %v writes %d", err, sess.writes)
	}
}

func TestEstimateTurnRateOverrideOnlyIsRecordedAsAMessage(t *testing.T) {
	sess, eng := newSess(), &fakeEstimateEngine{turn: okTurn()}
	send, _ := messenger(sess, eng, false)
	if _, err := send("", map[string]float64{"paint": 90000}); err != nil {
		t.Fatal(err)
	}
	if eng.got.overrides["paint"] != 90000 || sess.appended[0].Content != "Rates entered: paint = 90000" {
		t.Fatalf("%+v %+v", eng.got, sess.appended)
	}
}

func TestEstimateTurnWithoutAnEstimateStoresNone(t *testing.T) {
	sess := newSess()
	eng := &fakeEstimateEngine{turn: entity.EstimateTurn{State: json.RawMessage(`{"status":"interviewing"}`), Reply: "Where is it?"}}
	send, _ := messenger(sess, eng, false)
	if _, err := send("hi", nil); err != nil || sess.est != nil || sess.sess.Status() != "interviewing" {
		t.Fatalf("err %v est %v status %s", err, sess.est, sess.sess.Status())
	}
}

func TestEstimateTurnEmptyReplyIsAnEngineFailure(t *testing.T) {
	sess := newSess()
	eng := &fakeEstimateEngine{turn: entity.EstimateTurn{State: json.RawMessage(`{}`), Reply: " "}}
	send, _ := messenger(sess, eng, false)
	if _, err := send("hi", nil); !errors.Is(err, entity.ErrEngineUnavailable) || sess.writes != 0 {
		t.Fatalf("err %v writes %d", err, sess.writes)
	}
}

func TestEstimateTurnUnknownEngineStatusWritesNothing(t *testing.T) {
	sess := newSess()
	eng := &fakeEstimateEngine{turn: entity.EstimateTurn{State: json.RawMessage(`{"status":"weird"}`), Reply: "hi"}}
	send, _ := messenger(sess, eng, false)
	if _, err := send("hi", nil); !errors.Is(err, entity.ErrEngineUnavailable) || sess.writes != 0 {
		t.Fatalf("err %v writes %d", err, sess.writes)
	}
}

func TestStartingAnEstimateReplacesTheSessionOnce(t *testing.T) {
	sess := newSess()
	c := NewEstimateSessionCreator(&fakeBlueprints{brief: "null"}, sess, &fakeAuthz{})
	d, err := c.Create(context.Background(), "u", "t", bpID)
	if err != nil || len(sess.replaced) != 1 || d.Session.BlueprintID != bpID || d.Session.Status() != entity.EstimateInterviewing {
		t.Fatalf("err %v replaced %+v detail %+v", err, sess.replaced, d.Session)
	}
	if len(d.Messages) != 0 {
		t.Fatalf("a fresh session starts empty: %+v", d)
	}
}

func TestStartingAnEstimateNeedsEditAndTouchesNothingOtherwise(t *testing.T) {
	sess := newSess()
	c := NewEstimateSessionCreator(&fakeBlueprints{brief: "null"}, sess, &fakeAuthz{deny: true})
	if _, err := c.Create(context.Background(), "u", "t", bpID); !errors.Is(err, entity.ErrForbidden) || len(sess.replaced) != 0 {
		t.Fatalf("err %v replaced %d", err, len(sess.replaced))
	}
}

func TestAFailedReplaceReturnsTheErrorAndKeepsTheOldSession(t *testing.T) {
	sess := newSess()
	sess.replaceErr = errors.New("db down")
	before := sess.sess
	c := NewEstimateSessionCreator(&fakeBlueprints{brief: "null"}, sess, &fakeAuthz{})
	if _, err := c.Create(context.Background(), "u", "t", bpID); err == nil || sess.sess.Epoch != before.Epoch {
		t.Fatalf("err %v session %+v", err, sess.sess)
	}
}

func TestRestartingWhileATurnRunsMakesThatTurn409AndKeepsTheNewSession(t *testing.T) {
	sess := newSess()
	eng := &fakeEstimateEngine{turn: okTurn()}
	c := NewEstimateSessionCreator(&fakeBlueprints{brief: "null"}, sess, &fakeAuthz{})
	eng.during = func() { // the person starts over in another tab
		if _, err := c.Create(context.Background(), "u", "t", bpID); err != nil {
			t.Error(err)
		}
	}
	send, _ := messenger(sess, eng, false)
	if _, err := send("hi", nil); !errors.Is(err, entity.ErrConflict) || sess.writes != 0 {
		t.Fatalf("err %v writes %d", err, sess.writes)
	}
	if sess.sess.Epoch != 2 || sess.sess.Seq != 0 || len(sess.appended) != 0 || sess.est != nil {
		t.Fatalf("the new session must be untouched: %+v msgs %d", sess.sess, len(sess.appended))
	}
}

func TestATurnWithoutASessionIsNotFound(t *testing.T) {
	sess, eng := &fakeSessions{}, &fakeEstimateEngine{turn: okTurn()}
	send, _ := messenger(sess, eng, false)
	if _, err := send("hi", nil); !errors.Is(err, entity.ErrNotFound) || eng.calls != 0 {
		t.Fatalf("err %v calls %d", err, eng.calls)
	}
}
