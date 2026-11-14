// Package blueprintengine is the HTTP client for the stateless
// blueprint-engine sidecar (FastAPI). The engine has no auth; the services
// run their OpenFGA Checks before calling it.
package blueprintengine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"time"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
)

// Client calls the engine at BaseURL. LLM steps take tens of seconds on a
// laptop, so the timeout is generous.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

var _ gateway.BlueprintEngine = (*Client)(nil)

func New(baseURL string) *Client {
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), HTTP: &http.Client{Timeout: 5 * time.Minute}}
}

type draftOut struct {
	Brief json.RawMessage `json:"brief"`
	Notes []string        `json:"notes"`
}

type stepOut struct {
	Result struct {
		Brief  json.RawMessage `json:"brief"`
		Model  json.RawMessage `json:"model"`
		Checks json.RawMessage `json:"checks"`
		Issues []string        `json:"issues"`
		OK     bool            `json:"ok"`
		Notes  []string        `json:"notes"`
	} `json:"result"`
	Notes []string `json:"notes"`
}

func (o stepOut) outcome() entity.BlueprintOutcome {
	r := o.Result
	return entity.BlueprintOutcome{Brief: r.Brief, Model: r.Model, Checks: r.Checks, Issues: r.Issues, OK: r.OK,
		Notes: append(append([]string{}, o.Notes...), r.Notes...)}
}

func (c *Client) BriefFromText(ctx context.Context, prompt string) (entity.BlueprintDraft, error) {
	var out draftOut
	if err := c.postJSON(ctx, "/v1/brief/from-text", map[string]any{"prompt": prompt}, &out); err != nil {
		return entity.BlueprintDraft{}, err
	}
	return entity.BlueprintDraft{Brief: out.Brief, Notes: out.Notes}, nil
}

func (c *Client) BriefFromSketch(ctx context.Context, file entity.Upload, note string) (entity.BlueprintDraft, error) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename=%q`, file.Name))
	h.Set("Content-Type", file.MimeType)
	part, err := w.CreatePart(h)
	if err != nil {
		return entity.BlueprintDraft{}, err
	}
	if _, err := part.Write(file.Data); err != nil {
		return entity.BlueprintDraft{}, err
	}
	if err := w.WriteField("note", note); err != nil {
		return entity.BlueprintDraft{}, err
	}
	if err := w.Close(); err != nil {
		return entity.BlueprintDraft{}, err
	}
	var out draftOut
	if err := c.do(ctx, "/v1/brief/from-sketch", w.FormDataContentType(), &body, &out); err != nil {
		return entity.BlueprintDraft{}, err
	}
	return entity.BlueprintDraft{Brief: out.Brief, Notes: out.Notes}, nil
}

func (c *Client) Generate(ctx context.Context, brief, previous []byte, repair bool) (entity.BlueprintOutcome, error) {
	var out stepOut
	err := c.postJSON(ctx, "/v1/generate", map[string]any{"brief": json.RawMessage(brief), "previous": rawOrNull(previous), "repair": repair}, &out)
	return out.outcome(), err
}

func (c *Client) Revise(ctx context.Context, brief, previous []byte, message string, history []string) (entity.BlueprintOutcome, error) {
	var out stepOut
	err := c.postJSON(ctx, "/v1/revise", map[string]any{"brief": json.RawMessage(brief), "previous": rawOrNull(previous),
		"message": message, "history": nonNil(history)}, &out)
	return out.outcome(), err
}

func (c *Client) Review(ctx context.Context, brief, previous []byte, history []string) (entity.BlueprintOutcome, error) {
	var out stepOut
	err := c.postJSON(ctx, "/v1/review", map[string]any{"brief": json.RawMessage(brief), "previous": rawOrNull(previous),
		"history": nonNil(history)}, &out)
	return out.outcome(), err
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func (c *Client) Render(ctx context.Context, model []byte, format string, tb entity.BlueprintTitleBlock) (entity.Drawing, error) {
	in := map[string]any{"model": json.RawMessage(model), "format": format, "title_block": map[string]any{
		"project": tb.Project, "version": tb.Version, "issued": tb.Issued, "author": tb.Author}}
	b, err := json.Marshal(in)
	if err != nil {
		return entity.Drawing{}, err
	}
	resp, err := c.send(ctx, "/v1/render", "application/json", bytes.NewReader(b))
	if err != nil {
		return entity.Drawing{}, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 50<<20))
	if err != nil {
		return entity.Drawing{}, fmt.Errorf("%w: %v", entity.ErrEngineUnavailable, err)
	}
	return entity.Drawing{Format: format, ContentType: resp.Header.Get("Content-Type"), Data: data}, nil
}

func (c *Client) postJSON(ctx context.Context, path string, in, out any) error {
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}
	return c.do(ctx, path, "application/json", bytes.NewReader(b), out)
}

func (c *Client) do(ctx context.Context, path, contentType string, body io.Reader, out any) error {
	resp, err := c.send(ctx, path, contentType, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("%w: bad engine response: %v", entity.ErrEngineUnavailable, err)
	}
	return nil
}

// send returns the response on 2xx. A request the engine rejects (4xx) is
// ErrValidation with its message; anything else is ErrEngineUnavailable.
func (c *Client) send(ctx context.Context, path, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", entity.ErrEngineUnavailable, err)
	}
	if resp.StatusCode/100 == 2 {
		return resp, nil
	}
	defer resp.Body.Close()
	msg := errorMessage(resp.Body)
	if resp.StatusCode/100 == 4 {
		return nil, entity.Invalid("%s", msg)
	}
	return nil, fmt.Errorf("%w: engine %d: %s", entity.ErrEngineUnavailable, resp.StatusCode, msg)
}

// errorMessage reads the engine's {"message"} or FastAPI's {"detail": [...]}.
func errorMessage(r io.Reader) string {
	var e struct {
		Message string          `json:"message"`
		Detail  json.RawMessage `json:"detail"`
	}
	raw, _ := io.ReadAll(io.LimitReader(r, 64<<10))
	if json.Unmarshal(raw, &e) != nil {
		return strings.TrimSpace(string(raw))
	}
	if e.Message != "" {
		return e.Message
	}
	var details []struct {
		Loc []any  `json:"loc"`
		Msg string `json:"msg"`
	}
	if json.Unmarshal(e.Detail, &details) == nil && len(details) > 0 {
		parts := []string{}
		for i, d := range details {
			if i == 3 {
				break
			}
			loc := make([]string, 0, len(d.Loc))
			for _, l := range d.Loc[min(1, len(d.Loc)):] { // drop "body"
				loc = append(loc, fmt.Sprint(l))
			}
			parts = append(parts, strings.Join(loc, ".")+": "+d.Msg)
		}
		return "invalid brief: " + strings.Join(parts, "; ")
	}
	return strings.TrimSpace(string(e.Detail))
}

func rawOrNull(b []byte) json.RawMessage {
	if len(b) == 0 {
		return json.RawMessage("null")
	}
	return json.RawMessage(b)
}
