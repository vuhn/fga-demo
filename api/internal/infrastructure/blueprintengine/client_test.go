package blueprintengine

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
)

func TestErrorsMapToSentinels(t *testing.T) {
	cases := []struct {
		status int
		body   string
		want   error
		msg    string
	}{
		{422, `{"error":"invalid_edit","message":"no room called pool"}`, entity.ErrValidation, "no room called pool"},
		{422, `{"detail":[{"loc":["body","brief","lot","width"],"msg":"Input should be greater than 0"}]}`, entity.ErrValidation, "brief.lot.width: Input should be greater than 0"},
		{502, `{"error":"llm_unavailable","message":"ollama down"}`, entity.ErrEngineUnavailable, "ollama down"},
	}
	for _, tc := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.status)
			io.WriteString(w, tc.body)
		}))
		_, err := New(srv.URL).BriefFromText(context.Background(), "x")
		srv.Close()
		if !errors.Is(err, tc.want) || !strings.Contains(err.Error(), tc.msg) {
			t.Errorf("status %d: got %v, want %v containing %q", tc.status, err, tc.want, tc.msg)
		}
	}
	if _, err := New("http://127.0.0.1:1").BriefFromText(context.Background(), "x"); !errors.Is(err, entity.ErrEngineUnavailable) {
		t.Errorf("connection refused: got %v", err)
	}
}

func TestGenerateMergesNotesAndKeepsNullModel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(b), `"previous":null`) || !strings.Contains(string(b), `"repair":true`) {
			t.Errorf("request body %s", b)
		}
		io.WriteString(w, `{"result":{"brief":{},"model":null,"checks":[],"issues":["too big"],"ok":false,"notes":["b"]},"notes":["a"]}`)
	}))
	defer srv.Close()
	out, err := New(srv.URL).Generate(context.Background(), []byte(`{}`), nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if out.Built() || len(out.Issues) != 1 || strings.Join(out.Notes, ",") != "a,b" {
		t.Fatalf("outcome %+v", out)
	}
}

func TestReviseAndReviewSendHistory(t *testing.T) {
	var paths, bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		paths, bodies = append(paths, r.URL.Path), append(bodies, string(b))
		io.WriteString(w, `{"result":{"brief":{},"model":{},"checks":[],"issues":[],"ok":true},"notes":["review round 1: x"]}`)
	}))
	defer srv.Close()
	c := New(srv.URL)
	if _, err := c.Revise(context.Background(), []byte(`{}`), nil, "bigger", []string{"make the kitchen bigger"}); err != nil {
		t.Fatal(err)
	}
	out, err := c.Review(context.Background(), []byte(`{}`), []byte(`{}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(paths, ",") != "/v1/revise,/v1/review" {
		t.Fatalf("paths %v", paths)
	}
	if !strings.Contains(bodies[0], `"history":["make the kitchen bigger"]`) || !strings.Contains(bodies[1], `"history":[]`) {
		t.Errorf("bodies %v", bodies)
	}
	if len(out.Notes) != 1 {
		t.Errorf("notes %v", out.Notes)
	}
}

func TestReviseStreamRelaysProgressThenResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/revise/stream" {
			t.Errorf("path %s", r.URL.Path)
		}
		io.WriteString(w, `{"event":"progress","stage":"ask_model","message":"Asking the model… attempt 1/3"}`+"\n")
		io.WriteString(w, `{"event":"progress","stage":"checks","message":"Checks: all pass"}`+"\n")
		io.WriteString(w, `{"event":"result","data":{"result":{"brief":{},"model":{"a":1},"checks":[],"issues":[],"ok":true,"notes":["b"]},"notes":["a"]}}`+"\n")
	}))
	defer srv.Close()
	var got []string
	out, err := New(srv.URL).ReviseStream(context.Background(), []byte(`{}`), nil, "x", nil, func(p entity.BlueprintProgress) { got = append(got, p.Stage) })
	if err != nil || !out.Built() || strings.Join(got, ",") != "ask_model,checks" || strings.Join(out.Notes, ",") != "a,b" {
		t.Fatalf("got %v %v %+v", got, err, out)
	}
}

func TestReviseStreamErrorsMapToSentinels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"event":"progress","stage":"start","message":"s"}`+"\n")
		io.WriteString(w, `{"event":"error","status":422,"error":"invalid_edit","message":"no room called pool"}`+"\n")
	}))
	called := 0
	_, err := New(srv.URL).ReviseStream(context.Background(), []byte(`{}`), nil, "x", nil, func(entity.BlueprintProgress) { called++ })
	srv.Close()
	if !errors.Is(err, entity.ErrValidation) || !strings.Contains(err.Error(), "no room called pool") || called != 1 {
		t.Errorf("got %v after %d progress", err, called)
	}

	// a 4xx before the stream opens fails before any callback
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(422)
		io.WriteString(w, `{"error":"invalid_edit","message":"say what to change"}`)
	}))
	called = 0
	_, err = New(srv.URL).ReviseStream(context.Background(), []byte(`{}`), nil, "x", nil, func(entity.BlueprintProgress) { called++ })
	srv.Close()
	if !errors.Is(err, entity.ErrValidation) || called != 0 {
		t.Errorf("got %v after %d progress", err, called)
	}

	// a stream that ends without a result is an unavailable engine
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	_, err = New(srv.URL).ReviseStream(context.Background(), []byte(`{}`), nil, "x", nil, nil)
	srv.Close()
	if !errors.Is(err, entity.ErrEngineUnavailable) {
		t.Errorf("empty stream: got %v", err)
	}
}
