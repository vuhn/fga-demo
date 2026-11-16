"""DXF import of files that are not floor plans: every entity is kept as a read-only drawing item."""

from __future__ import annotations

import io
from collections import Counter
from pathlib import Path

import ezdxf
import pytest
from fastapi.testclient import TestClient

from blueprint_engine.app import create_app
from blueprint_engine.checks import run_imported_checks
from blueprint_engine.draw import build_dxf, to_dxf_bytes, to_pdf, to_svg
from blueprint_engine.dxf_patch import patch_dxf
from blueprint_engine.dxf_plan import import_dxf
from blueprint_engine.edits import EditError
from blueprint_engine.ifc import to_ifc
from blueprint_engine.schemas import BuildingModel

SAMPLES = sorted((Path(__file__).parent / "samples" / "jscad").glob("*.dxf"))  # jscad/sample-files, dxf-parser


def _bytes(doc) -> bytes:
    buf = io.StringIO()
    doc.write(buf)
    return buf.getvalue().encode()


@pytest.mark.parametrize("path", SAMPLES, ids=lambda p: p.name)
def test_every_sample_file_imports_and_renders(path):
    data = path.read_bytes()
    res = import_dxf(data)
    assert res.plans and res.report.plans_found >= 1
    for plan in res.plans:
        m = plan.model
        assert m.drawing or m.rooms
        again = BuildingModel.model_validate_json(m.model_dump_json())  # what the API stores and sends back
        assert again == m
        doc, sheet = build_dxf(m)
        assert "<svg" in to_svg(doc, sheet) and to_dxf_bytes(doc) and to_pdf(doc, sheet).startswith(b"%PDF")
        to_ifc(m)
        if not m.rooms:
            assert run_imported_checks(m) == [] and plan.room_count == 0
        out, _ = patch_dxf(data, m, m)  # nothing edited: the original comes back byte for byte
        assert out == data


def test_circles_arcs_ellipses_and_splines_become_curves():
    res = import_dxf((Path(__file__).parent / "samples" / "jscad" / "circlesellipsesarcs.dxf").read_bytes())
    kinds = Counter(i.type for i in res.plans[0].model.drawing)
    assert kinds == {"ELLIPSE": 2, "ARC": 2, "CIRCLE": 1}
    circle = next(i for i in res.plans[0].model.drawing if i.type == "CIRCLE")
    assert len(circle.paths[0]) > 16 and circle.paths[0][0] == circle.paths[0][-1]  # a closed, flattened curve


def test_all_entity_kinds_in_one_file():
    doc = ezdxf.new("R2010", setup=True)
    doc.header["$INSUNITS"] = 4  # mm
    doc.layers.add("HIDDEN").off()
    msp = doc.modelspace()
    msp.add_line((0, 0), (100, 0))
    msp.add_circle((50, 50), 20)
    msp.add_arc((50, 50), 30, 0, 90)
    msp.add_ellipse((50, 50), major_axis=(40, 0), ratio=0.5)
    msp.add_spline([(0, 0), (20, 40), (60, 40), (100, 0)])
    msp.add_lwpolyline([(0, 0), (10, 0), (10, 10)], close=True)
    msp.add_polyline3d([(0, 0, 0), (10, 10, 5), (20, 0, 9)])
    msp.add_point((5, 5))
    msp.add_text("hello", height=5).set_placement((10, 10))
    msp.add_mtext("line one\\Pline two", dxfattribs={"char_height": 4, "insert": (30, 30)})
    hatch = msp.add_hatch(color=1)
    hatch.paths.add_polyline_path([(0, 0), (30, 0), (30, 30), (0, 30)], is_closed=True)
    msp.add_solid([(0, 0), (10, 0), (0, 10), (10, 10)])
    msp.add_3dface([(0, 0, 0), (10, 0, 0), (10, 10, 0), (0, 10, 0)])
    msp.add_linear_dim(base=(0, -20), p1=(0, 0), p2=(100, 0), dimstyle="EZDXF").render()
    msp.add_leader([(0, 0), (20, 20), (40, 20)])
    blk = doc.blocks.new("INNER")
    blk.add_circle((0, 0), 3)
    outer = doc.blocks.new("OUTER")
    outer.add_line((0, 0), (5, 5))
    outer.add_blockref("INNER", (10, 0))
    msp.add_blockref("OUTER", (200, 200))
    msp.add_line((0, 0), (50, 50), dxfattribs={"layer": "HIDDEN"})
    msp.add_ray((0, 0), (1, 1))

    res = import_dxf(_bytes(doc))
    assert len(res.plans) == 1
    plan = res.plans[0]
    kinds = Counter(i.type for i in plan.model.drawing)
    for kind in ("LINE", "CIRCLE", "ARC", "ELLIPSE", "SPLINE", "LWPOLYLINE", "POLYLINE", "POINT", "TEXT", "MTEXT",
                 "HATCH", "SOLID", "3DFACE"):
        assert kinds[kind] >= 1, (kind, dict(kinds))
    assert kinds["CIRCLE"] >= 2  # the circle and the one nested two blocks deep
    texts = {i.text for i in plan.model.drawing if i.at and i.text}
    assert "hello" in texts and any("line two" in t for t in texts) and any(t.strip() for t in texts if t not in ("hello",))
    assert any("hidden layer" in s for s in res.report.skipped) and any("infinite ray" in s for s in res.report.skipped)
    assert all(i.layer != "HIDDEN" for i in plan.model.drawing)
    assert res.report.units == "mm" and plan.room_count == 0
    # millimetres in the file, metres in the model: the 100 mm line is 0.1 m
    x0, y0, x1, y1 = plan.model.footprint
    assert x0 == 0.0 and y0 == 0.0 and 0.2 < x1 < 0.5 and plan.model.origin is not None and plan.model.origin.scale == 0.001


def test_a_floor_plan_keeps_its_rooms_and_gets_the_other_entities_as_drawing_items():
    import sample_dxf

    data, _ = sample_dxf.sheet()
    res = import_dxf(data)
    assert all(p.room_count > 0 for p in res.plans)
    used = set(res.report.layers_used)
    for p in res.plans:
        assert all(i.layer not in used for i in p.model.drawing)  # walls, doors, windows are not drawn twice


def test_a_file_with_no_entities_is_refused():
    data = _bytes(ezdxf.new("R2010"))
    with pytest.raises(EditError):
        import_dxf(data)


def test_endpoint_imports_a_plain_drawing():
    path = Path(__file__).parent / "samples" / "jscad" / "splines.dxf"
    c = TestClient(create_app(llm=object()))
    r = c.post("/v1/import/dxf", files={"file": (path.name, path.read_bytes(), "image/vnd.dxf")})
    assert r.status_code == 200, r.text
    body = r.json()
    plan = body["plans"][0]
    assert plan["room_count"] == 0 and plan["ok"] and plan["checks"] == [] and "<svg" in plan["svg"]
    assert len(plan["model"]["drawing"]) == 2 and body["report"]["plans_found"] == 1
    assert any("no floor plan" in w for w in body["report"]["warnings"])


@pytest.mark.parametrize("path", [p for p in SAMPLES if p.name != "blocks2.dxf"], ids=lambda p: p.name)
def test_a_drawing_fills_its_sheet(path):
    """A drawing of any size is drawn at the largest scale that fits, not a dot at 1:50 (a 10 mm part, a site)."""
    from blueprint_engine.draw import PAPERS, _scale_for

    m = import_dxf(path.read_bytes()).plans[0].model
    if m.rooms:
        return
    denom, paper = _scale_for(m)
    x0, y0, x1, y1 = m.footprint
    pw, ph = PAPERS[paper]
    assert max((x1 - x0) / (pw * denom), (y1 - y0) / (ph * denom)) > 0.15, (denom, paper, m.footprint)
