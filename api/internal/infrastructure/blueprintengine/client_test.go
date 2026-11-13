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
