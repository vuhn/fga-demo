// Package engine is the gRPC client of the deterministic blueprint Engine (:8092). It implements
// gateway.BlueprintEngine. The Engine is stateless, has no auth and never sees a tenant or a person: the
// services run their OpenFGA Checks before calling it.
package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/infrastructure/grpcutil"
	commonv1 "github.com/vuhn/fga-demo/proto/go/fga/demo/common/v1"
	enginev1 "github.com/vuhn/fga-demo/proto/go/fga/demo/engine/v1"
)

// DefaultTimeout bounds a call that carries no deadline of its own. Conversion of a scan runs OCR, so it is
// generous.
const DefaultTimeout = 5 * time.Minute

const service = "engine"

// Client calls the Engine over gRPC.
type Client struct {
	rpc enginev1.EngineServiceClient
}

var _ gateway.BlueprintEngine = (*Client)(nil)

// New wraps an open connection (see Dial).
func New(cc grpc.ClientConnInterface) *Client {
	return &Client{rpc: enginev1.NewEngineServiceClient(cc)}
}

// Dial connects to addr. Connecting is lazy: an Engine that is down surfaces as ErrEngineUnavailable on the
// first call, not at start-up.
func Dial(addr string) (*Client, *grpc.ClientConn, error) {
	cc, err := grpcutil.Dial(addr, DefaultTimeout)
	if err != nil {
		return nil, nil, err
	}
	return New(cc), cc, nil
}

func call[R any](fn func(opt grpc.CallOption) (R, error)) (R, error) {
	return grpcutil.Call(service, func(tr *metadata.MD) (R, error) { return fn(grpc.Trailer(tr)) })
}

func (c *Client) Generate(ctx context.Context, brief, previous []byte, repair bool) (model.BlueprintOutcome, error) {
	r, err := call(func(o grpc.CallOption) (*enginev1.BriefResult, error) {
		return c.rpc.Generate(ctx, &enginev1.GenerateRequest{BriefJson: brief, PreviousModelJson: previous, Repair: repair}, o)
	})
	if err != nil {
		return model.BlueprintOutcome{}, err
	}
	return model.BlueprintOutcome{Brief: raw(r.BriefJson), Model: raw(r.ModelJson), Checks: grpcutil.ChecksJSON(r.Checks),
		Issues: r.Issues, OK: r.Ok, Notes: r.Notes}, nil
}

func (c *Client) CreatePlan(ctx context.Context, brief []byte) (model.BlueprintOutcome, error) {
	r, err := call(func(o grpc.CallOption) (*enginev1.PlanOutcome, error) {
		return c.rpc.CreatePlan(ctx, &enginev1.CreatePlanRequest{BriefJson: brief}, o)
	})
	return planOutcome(r, err)
}

func (c *Client) DrawPlan(ctx context.Context, plan []byte, title string) (model.BlueprintOutcome, error) {
	r, err := call(func(o grpc.CallOption) (*enginev1.PlanOutcome, error) {
		return c.rpc.DrawPlan(ctx, &enginev1.DrawPlanRequest{ModelJson: plan, Title: title}, o)
	})
	return planOutcome(r, err)
}

func planOutcome(r *enginev1.PlanOutcome, err error) (model.BlueprintOutcome, error) {
	if err != nil {
		return model.BlueprintOutcome{}, err
	}
	return model.BlueprintOutcome{Model: raw(r.ModelJson), Checks: grpcutil.ChecksJSON(r.Checks), Issues: r.Issues, OK: r.Ok,
		Notes: r.Notes, DXF: r.Dxf}, nil
}

func (c *Client) EditPlan(ctx context.Context, plan, ops []byte) (model.BlueprintOutcome, error) {
	r, err := call(func(o grpc.CallOption) (*enginev1.PlanStep, error) {
		return c.rpc.ApplyPlanOps(ctx, &enginev1.ApplyPlanOpsRequest{ModelJson: plan, OpsJson: ops}, o)
	})
	if err != nil {
		return model.BlueprintOutcome{}, err
	}
	return model.BlueprintOutcome{Model: raw(r.ModelJson), Checks: grpcutil.ChecksJSON(r.Checks), OK: r.Ok, Notes: r.Notes}, nil
}

func (c *Client) PatchDXF(ctx context.Context, original model.Upload, base, current []byte) (model.PatchedFile, error) {
	r, err := call(func(o grpc.CallOption) (*enginev1.PatchDXFResponse, error) {
		return c.rpc.PatchDXF(ctx, &enginev1.PatchDXFRequest{Original: upload(original), BaseModelJson: base, CurrentModelJson: current}, o)
	})
	if err != nil {
		return model.PatchedFile{}, err
	}
	if len(r.Dxf) == 0 {
		return model.PatchedFile{}, fmt.Errorf("%w: bad engine response: no file", model.ErrEngineUnavailable)
	}
	return model.PatchedFile{Name: original.Name, MimeType: original.MimeType, Data: r.Dxf, Report: raw(r.ReportJson)}, nil
}

func (c *Client) ConvertToDXF(ctx context.Context, file model.Upload, opts model.ConvertOptions) (model.BlueprintConversion, error) {
	r, err := call(func(o grpc.CallOption) (*enginev1.ConvertToDXFResponse, error) {
		return c.rpc.ConvertToDXF(ctx, &enginev1.ConvertToDXFRequest{File: upload(file),
			Options: &enginev1.ConvertOptions{Scale: int32(opts.Scale), LongestM: opts.LongestM}}, o)
	})
	if err != nil {
		return model.BlueprintConversion{}, err
	}
	return model.BlueprintConversion{DXF: r.Dxf, Report: raw(r.ReportJson)}, nil
}

func (c *Client) ImportDXF(ctx context.Context, file model.Upload, unit string) (model.BlueprintImport, error) {
	r, err := call(func(o grpc.CallOption) (*enginev1.ImportDXFResponse, error) {
		return c.rpc.ImportDXF(ctx, &enginev1.ImportDXFRequest{File: upload(file), Unit: unit}, o)
	})
	if err != nil {
		return model.BlueprintImport{}, err
	}
	var plans []struct {
		Index     int             `json:"index"`
		Name      string          `json:"name"`
		RoomCount int             `json:"room_count"`
		NetArea   float64         `json:"net_area"`
		OK        bool            `json:"ok"`
		Model     json.RawMessage `json:"model"`
		Checks    json.RawMessage `json:"checks"`
		SVG       string          `json:"svg"`
	}
	if err := json.Unmarshal(r.PlansJson, &plans); err != nil {
		return model.BlueprintImport{}, fmt.Errorf("%w: bad engine response: %v", model.ErrEngineUnavailable, err)
	}
	res := model.BlueprintImport{Report: raw(r.ReportJson)}
	for _, p := range plans {
		res.Plans = append(res.Plans, model.BlueprintImportPlan{Index: p.Index, Name: p.Name, RoomCount: p.RoomCount,
			NetArea: p.NetArea, OK: p.OK, Model: p.Model, Checks: p.Checks, SVG: p.SVG})
	}
	return res, nil
}

var formats = map[string]enginev1.RenderFormat{
	model.DrawingSVG: enginev1.RenderFormat_RENDER_FORMAT_SVG,
	model.DrawingPDF: enginev1.RenderFormat_RENDER_FORMAT_PDF,
	model.DrawingDXF: enginev1.RenderFormat_RENDER_FORMAT_DXF,
	model.DrawingIFC: enginev1.RenderFormat_RENDER_FORMAT_IFC,
}

func (c *Client) Render(ctx context.Context, plan []byte, format string, tb model.BlueprintTitleBlock) (model.Drawing, error) {
	f, ok := formats[format]
	if !ok {
		return model.Drawing{}, model.Invalid("unknown render format %q", format)
	}
	r, err := call(func(o grpc.CallOption) (*enginev1.RenderResponse, error) {
		return c.rpc.Render(ctx, &enginev1.RenderRequest{ModelJson: plan, Format: f, TitleBlock: &enginev1.TitleBlock{
			Title: tb.Project, Version: int32(tb.Version), Issued: tb.Issued, Author: tb.Author}}, o)
	})
	if err != nil {
		return model.Drawing{}, err
	}
	return model.Drawing{Format: format, ContentType: r.MediaType, Data: r.Data}, nil
}

func upload(u model.Upload) *commonv1.Upload {
	return &commonv1.Upload{Name: u.Name, Mime: u.MimeType, Data: u.Data}
}

// raw keeps an empty JSON field absent (nil) rather than an empty-but-non-nil message.
func raw(b []byte) json.RawMessage {
	if len(b) == 0 {
		return nil
	}
	return json.RawMessage(b)
}
