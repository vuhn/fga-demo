package mapper

import (
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

func BlueprintView(b output.Blueprint) map[string]any {
	var source any
	if b.Source != nil {
		source = map[string]any{"name": b.Source.Name, "mime_type": b.Source.MimeType, "byte_size": b.Source.ByteSize}
	}
	return map[string]any{
		"id": b.ID, "tenant_id": b.TenantID, "project_id": b.ProjectID, "name": b.Name, "current_version": b.CurrentVersion,
		"source_kind": b.SourceKind, "source": source, "created_by": b.CreatedBy, "created_at": b.CreatedAt, "updated_at": b.UpdatedAt,
	}
}

func VisibleBlueprintView(v output.Visible[output.Blueprint]) map[string]any {
	m := BlueprintView(v.Value)
	m["permissions"] = v.Perms
	return m
}

// BlueprintVersionSummary is a version without its JSON payloads.
func BlueprintVersionSummary(v output.BlueprintVersion) map[string]any {
	return map[string]any{
		"version": v.Version, "kind": v.Kind, "message": v.Message, "ok": v.OK, "created_by": v.CreatedBy, "created_at": v.CreatedAt,
	}
}

func BlueprintVersionView(v output.BlueprintVersion) map[string]any {
	m := BlueprintVersionSummary(v)
	m["brief"], m["model"], m["checks"], m["notes"] = v.Brief, v.Model, v.Checks, v.Notes
	return m
}

func BlueprintDetailView(d output.BlueprintDetail) map[string]any {
	m := BlueprintView(d.Blueprint)
	if d.Perms != nil {
		m["permissions"] = d.Perms
	}
	m["version"] = BlueprintVersionView(d.Version)
	return m
}

func DraftView(d output.BlueprintDraft) map[string]any {
	notes := d.Notes
	if notes == nil {
		notes = []string{}
	}
	return map[string]any{"brief": d.Brief, "notes": notes}
}
