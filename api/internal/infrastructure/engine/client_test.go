package engine

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"

	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/infrastructure/grpcutil"
	commonv1 "github.com/vuhn/fga-demo/proto/go/fga/demo/common/v1"
	enginev1 "github.com/vuhn/fga-demo/proto/go/fga/demo/engine/v1"
)

// fake is an in-process Engine; each test sets only the handler it needs.
type fake struct {
	enginev1.UnimplementedEngineServiceServer
	generate func(context.Context, *enginev1.GenerateRequest) (*enginev1.BriefResult, error)
	importer func(context.Context, *enginev1.ImportDXFRequest) (*enginev1.ImportDXFResponse, error)
	render   func(context.Context, *enginev1.RenderRequest) (*enginev1.RenderResponse, error)
	convert  func(context.Context, *enginev1.ConvertToDXFRequest) (*enginev1.ConvertToDXFResponse, error)
	patch    func(context.Context, *enginev1.PatchDXFRequest) (*enginev1.PatchDXFResponse, error)
	create   func(context.Context, *enginev1.CreatePlanRequest) (*enginev1.PlanOutcome, error)
	edit     func(context.Context, *enginev1.ApplyPlanOpsRequest) (*enginev1.PlanStep, error)
}

func (f *fake) Generate(ctx context.Context, r *enginev1.GenerateRequest) (*enginev1.BriefResult, error) {
	return f.generate(ctx, r)
}
func (f *fake) ImportDXF(ctx context.Context, r *enginev1.ImportDXFRequest) (*enginev1.ImportDXFResponse, error) {
	return f.importer(ctx, r)
}
func (f *fake) Render(ctx context.Context, r *enginev1.RenderRequest) (*enginev1.RenderResponse, error) {
	return f.render(ctx, r)
}
func (f *fake) ConvertToDXF(ctx context.Context, r *enginev1.ConvertToDXFRequest) (*enginev1.ConvertToDXFResponse, error) {
	return f.convert(ctx, r)
}
func (f *fake) PatchDXF(ctx context.Context, r *enginev1.PatchDXFRequest) (*enginev1.PatchDXFResponse, error) {
	return f.patch(ctx, r)
}
func (f *fake) CreatePlan(ctx context.Context, r *enginev1.CreatePlanRequest) (*enginev1.PlanOutcome, error) {
	return f.create(ctx, r)
}
func (f *fake) ApplyPlanOps(ctx context.Context, r *enginev1.ApplyPlanOpsRequest) (*enginev1.PlanStep, error) {
	return f.edit(ctx, r)
}

// serve starts f behind a buffer connection with the real client options (message limits, interceptors).
func serve(t *testing.T, f *fake, timeout time.Duration) *Client {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer(grpc.MaxRecvMsgSize(grpcutil.MaxMessage), grpc.MaxSendMsgSize(grpcutil.MaxMessage))
	enginev1.RegisterEngineServiceServer(srv, f)
	go func() { _ = srv.Serve(lis) }()
	cc, err := grpcutil.Dial("passthrough:///bufnet", timeout,
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cc.Close(); srv.Stop() })
	return New(cc)
}

// refuse is an INVALID_ARGUMENT with an ErrorDetail in the trailer, as the Engine sends it.
func refuse(ctx context.Context, code commonv1.ErrorCode, msg string) error {
	b, _ := proto.Marshal(&commonv1.ErrorDetail{Code: code, Message: msg})
	_ = grpc.SetTrailer(ctx, metadata.Pairs(grpcutil.ErrorDetailKey, string(b)))
	return status.Error(codes.InvalidArgument, "refused")
}

func TestGenerateMapsTheResult(t *testing.T) {
	var gotID string
	f := &fake{generate: func(ctx context.Context, r *enginev1.GenerateRequest) (*enginev1.BriefResult, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		gotID = strings.Join(md.Get(grpcutil.RequestIDKey), ",")
		if string(r.BriefJson) != `{"a":1}` || string(r.PreviousModelJson) != `{"m":1}` || !r.Repair {
			t.Errorf("request not forwarded: %+v", r)
		}
		return &enginev1.BriefResult{BriefJson: []byte(`{"a":2}`), ModelJson: []byte(`{"m":2}`), Ok: true,
			Issues: []string{"i"}, Notes: []string{"n"}, Checks: []*commonv1.Check{
				{Rule: "r1", Severity: "error", Ok: false, Message: "m", RoomId: "room1", Category: "geometry", IssueId: "r1:room1", Hint: "h"},
				{Rule: "r2", Severity: "warning", Ok: true, Message: "m2", Category: "program"}}}, nil
	}}
	c := serve(t, f, time.Minute)
	ctx := grpcutil.WithRequestID(context.Background(), "req-1")
	out, err := c.Generate(ctx, []byte(`{"a":1}`), []byte(`{"m":1}`), true)
	if err != nil {
		t.Fatal(err)
	}
	if gotID != "req-1" {
		t.Errorf("x-request-id = %q", gotID)
	}
	if string(out.Brief) != `{"a":2}` || string(out.Model) != `{"m":2}` || !out.OK || out.Issues[0] != "i" || out.Notes[0] != "n" {
		t.Errorf("outcome = %+v", out)
	}
	var checks []map[string]any
	if err := json.Unmarshal(out.Checks, &checks); err != nil || len(checks) != 2 {
		t.Fatalf("checks = %s (%v)", out.Checks, err)
	}
	if checks[0]["room_id"] != "room1" || checks[1]["room_id"] != nil {
		t.Errorf("room_id shape: %v", checks)
	}
	if els, ok := checks[1]["elements"].([]any); !ok || len(els) != 0 {
		t.Errorf("elements must be [] not null: %v", checks[1]["elements"])
	}
}

func TestAnIdIsMadeWhenTheCallerHasNone(t *testing.T) {
	var gotID string
	f := &fake{generate: func(ctx context.Context, _ *enginev1.GenerateRequest) (*enginev1.BriefResult, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		gotID = strings.Join(md.Get(grpcutil.RequestIDKey), ",")
		return &enginev1.BriefResult{}, nil
	}}
	if _, err := serve(t, f, time.Minute).Generate(context.Background(), nil, nil, false); err != nil {
		t.Fatal(err)
	}
	if len(gotID) != 16 {
		t.Errorf("generated id = %q", gotID)
	}
}

func TestStatusMapping(t *testing.T) {
	cases := []struct {
		name    string
		err     func(context.Context) error
		invalid bool
		want    string
	}{
		{"edit error", func(ctx context.Context) error {
			return refuse(ctx, commonv1.ErrorCode_ERROR_CODE_EDIT_ERROR, "area too small")
		}, true, "area too small"},
		{"unsupported", func(ctx context.Context) error {
			return refuse(ctx, commonv1.ErrorCode_ERROR_CODE_UNSUPPORTED, "no such op")
		}, true, "no such op"},
		{"bad file", func(ctx context.Context) error {
			return refuse(ctx, commonv1.ErrorCode_ERROR_CODE_BAD_FILE, "not a DXF")
		}, true, "not a DXF"},
		{"invalid without detail", func(context.Context) error { return status.Error(codes.InvalidArgument, "bare") }, true, "bare"},
		{"unavailable", func(context.Context) error { return status.Error(codes.Unavailable, "down") }, false, "down"},
		{"internal", func(context.Context) error { return status.Error(codes.Internal, "boom") }, false, "boom"},
		{"unimplemented", func(context.Context) error { return status.Error(codes.Unimplemented, "old engine") }, false, "old engine"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &fake{generate: func(ctx context.Context, _ *enginev1.GenerateRequest) (*enginev1.BriefResult, error) {
				return nil, tc.err(ctx)
			}}
			_, err := serve(t, f, time.Minute).Generate(context.Background(), nil, nil, false)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to say %q", err, tc.want)
			}
			if tc.invalid != errors.Is(err, model.ErrValidation) {
				t.Errorf("ErrValidation = %v, want %v (%v)", errors.Is(err, model.ErrValidation), tc.invalid, err)
			}
			if !tc.invalid && !errors.Is(err, model.ErrEngineUnavailable) {
				t.Errorf("want ErrEngineUnavailable, got %v", err)
			}
		})
	}
}

func TestADownEngineIsUnavailable(t *testing.T) {
	c, cc, err := Dial("127.0.0.1:1")
	if err != nil {
		t.Fatal(err)
	}
	defer cc.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := c.Generate(ctx, nil, nil, false); !errors.Is(err, model.ErrEngineUnavailable) {
		t.Fatalf("err = %v", err)
	}
}

func TestDeadline(t *testing.T) {
	slow := &fake{generate: func(ctx context.Context, _ *enginev1.GenerateRequest) (*enginev1.BriefResult, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	t.Run("default applies when the caller has none", func(t *testing.T) {
		start := time.Now()
		_, err := serve(t, slow, 100*time.Millisecond).Generate(context.Background(), nil, nil, false)
		if !errors.Is(err, model.ErrEngineUnavailable) || time.Since(start) > 3*time.Second {
			t.Fatalf("err = %v after %v", err, time.Since(start))
		}
	})
	t.Run("the caller's shorter deadline wins", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		start := time.Now()
		_, err := serve(t, slow, time.Hour).Generate(ctx, nil, nil, false)
		if !errors.Is(err, model.ErrEngineUnavailable) || time.Since(start) > 3*time.Second {
			t.Fatalf("err = %v after %v", err, time.Since(start))
		}
	})
}

func TestALargeUploadAndAnswerCrossTheWire(t *testing.T) {
	big := make([]byte, 20<<20+1)
	for i := range big {
		big[i] = byte(i)
	}
	f := &fake{importer: func(_ context.Context, r *enginev1.ImportDXFRequest) (*enginev1.ImportDXFResponse, error) {
		if len(r.File.Data) != len(big) || r.File.Name != "plan.dxf" || r.Unit != "mm" {
			t.Errorf("upload = %d bytes %q %q", len(r.File.Data), r.File.Name, r.Unit)
		}
		plans, _ := json.Marshal([]map[string]any{{"index": 0, "name": "Ground", "room_count": 3, "net_area": 41.5, "ok": true,
			"model": map[string]any{"rooms": []any{}}, "checks": []any{}, "svg": "<svg/>"}})
		return &enginev1.ImportDXFResponse{PlansJson: plans, ReportJson: []byte(`{"skipped":0}`)}, nil
	}}
	res, err := serve(t, f, time.Minute).ImportDXF(context.Background(), model.Upload{Name: "plan.dxf", MimeType: "application/dxf", Data: big}, "mm")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Plans) != 1 || res.Plans[0].Name != "Ground" || res.Plans[0].RoomCount != 3 || res.Plans[0].NetArea != 41.5 ||
		res.Plans[0].SVG != "<svg/>" || string(res.Report) != `{"skipped":0}` {
		t.Errorf("import = %+v", res)
	}
}

func TestImportRefusesAnUnreadableFile(t *testing.T) {
	f := &fake{importer: func(ctx context.Context, _ *enginev1.ImportDXFRequest) (*enginev1.ImportDXFResponse, error) {
		return nil, refuse(ctx, commonv1.ErrorCode_ERROR_CODE_BAD_FILE, "file is too large")
	}}
	_, err := serve(t, f, time.Minute).ImportDXF(context.Background(), model.Upload{Name: "x.dxf"}, "")
	if !errors.Is(err, model.ErrValidation) || !strings.Contains(err.Error(), "file is too large") {
		t.Fatalf("err = %v", err)
	}
}

func TestRender(t *testing.T) {
	f := &fake{render: func(_ context.Context, r *enginev1.RenderRequest) (*enginev1.RenderResponse, error) {
		if r.Format != enginev1.RenderFormat_RENDER_FORMAT_PDF || r.TitleBlock.Title != "House" || r.TitleBlock.Version != 3 ||
			r.TitleBlock.Issued != "2026-11-19" || r.TitleBlock.Author != "ada" {
			t.Errorf("request = %+v", r)
		}
		return &enginev1.RenderResponse{Data: []byte("%PDF"), MediaType: "application/pdf"}, nil
	}}
	c := serve(t, f, time.Minute)
	d, err := c.Render(context.Background(), []byte(`{}`), model.DrawingPDF, model.BlueprintTitleBlock{Project: "House", Version: 3, Issued: "2026-11-19", Author: "ada"})
	if err != nil || d.Format != "pdf" || d.ContentType != "application/pdf" || string(d.Data) != "%PDF" {
		t.Fatalf("drawing = %+v err %v", d, err)
	}
	if _, err := c.Render(context.Background(), nil, "bmp", model.BlueprintTitleBlock{}); !errors.Is(err, model.ErrValidation) {
		t.Errorf("unknown format err = %v", err)
	}
}

func TestConvertPatchCreateAndEdit(t *testing.T) {
	f := &fake{
		convert: func(_ context.Context, r *enginev1.ConvertToDXFRequest) (*enginev1.ConvertToDXFResponse, error) {
			if r.Options.Scale != 100 || r.Options.LongestM != 12.5 || r.File.Mime != "image/png" {
				t.Errorf("convert request = %+v", r)
			}
			return &enginev1.ConvertToDXFResponse{Dxf: []byte("DXF"), ReportJson: []byte(`{"w":[]}`)}, nil
		},
		patch: func(_ context.Context, r *enginev1.PatchDXFRequest) (*enginev1.PatchDXFResponse, error) {
			if string(r.BaseModelJson) != "b" || string(r.CurrentModelJson) != "c" || r.Original.Name != "o.dxf" {
				t.Errorf("patch request = %+v", r)
			}
			return &enginev1.PatchDXFResponse{Dxf: []byte("PATCHED"), ReportJson: []byte(`{"applied":1}`)}, nil
		},
		create: func(_ context.Context, r *enginev1.CreatePlanRequest) (*enginev1.PlanOutcome, error) {
			return &enginev1.PlanOutcome{ModelJson: []byte(`{"m":1}`), Ok: true, Dxf: []byte("DRAWN"), Notes: []string{"n"}}, nil
		},
		edit: func(_ context.Context, r *enginev1.ApplyPlanOpsRequest) (*enginev1.PlanStep, error) {
			return &enginev1.PlanStep{ModelJson: []byte(`{"m":2}`), Ok: true, Notes: []string{"moved"}}, nil
		},
	}
	c := serve(t, f, time.Minute)
	ctx := context.Background()

	conv, err := c.ConvertToDXF(ctx, model.Upload{Name: "a.png", MimeType: "image/png", Data: []byte("x")}, model.ConvertOptions{Scale: 100, LongestM: 12.5})
	if err != nil || string(conv.DXF) != "DXF" || string(conv.Report) != `{"w":[]}` {
		t.Errorf("convert = %+v %v", conv, err)
	}
	p, err := c.PatchDXF(ctx, model.Upload{Name: "o.dxf", MimeType: "application/dxf", Data: []byte("o")}, []byte("b"), []byte("c"))
	if err != nil || string(p.Data) != "PATCHED" || p.Name != "o.dxf" || p.MimeType != "application/dxf" || string(p.Report) != `{"applied":1}` {
		t.Errorf("patch = %+v %v", p, err)
	}
	cr, err := c.CreatePlan(ctx, []byte(`{}`))
	if err != nil || string(cr.DXF) != "DRAWN" || cr.Brief != nil || string(cr.Model) != `{"m":1}` || !cr.OK {
		t.Errorf("create = %+v %v", cr, err)
	}
	ed, err := c.EditPlan(ctx, []byte(`{}`), []byte(`[]`))
	if err != nil || string(ed.Model) != `{"m":2}` || ed.Notes[0] != "moved" {
		t.Errorf("edit = %+v %v", ed, err)
	}
}

func TestPatchWithoutAFileIsAnEngineFault(t *testing.T) {
	f := &fake{patch: func(context.Context, *enginev1.PatchDXFRequest) (*enginev1.PatchDXFResponse, error) {
		return &enginev1.PatchDXFResponse{}, nil
	}}
	_, err := serve(t, f, time.Minute).PatchDXF(context.Background(), model.Upload{Name: "o.dxf"}, nil, nil)
	if !errors.Is(err, model.ErrEngineUnavailable) {
		t.Fatalf("err = %v", err)
	}
}

func TestInBandErrors(t *testing.T) {
	if err := grpcutil.InBand("agent", &commonv1.ErrorDetail{Code: commonv1.ErrorCode_ERROR_CODE_DEPENDENCY_UNAVAILABLE, Message: "llm down", Dependency: "llm"}); !errors.Is(err, model.ErrEngineUnavailable) {
		t.Errorf("dependency err = %v", err)
	}
	if err := grpcutil.InBand("agent", &commonv1.ErrorDetail{Code: commonv1.ErrorCode_ERROR_CODE_UNSUPPORTED, Message: "nope"}); !errors.Is(err, model.ErrValidation) {
		t.Errorf("refusal err = %v", err)
	}
	if grpcutil.InBand("agent", nil) != nil {
		t.Error("nil detail must be nil")
	}
}
