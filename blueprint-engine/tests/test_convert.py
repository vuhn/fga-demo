"""PDF / PNG -> DXF -> import, against the sample plans' known areas (our own exports, so the answer is known)."""

from pathlib import Path

import pytest
from fastapi.testclient import TestClient

from fga_engine.infrastructure import convert, dxf_plan
from fga_engine.app import create_app
from fga_engine.domain.edits import EditError

OUT = Path(__file__).parent.parent / "out"
# name: (paper scale of the sheet, longest side in m, net area in m² from the original DXF)
CASES = {"family_3br": (50, 16.9, 109.4), "studio": (100, 9.5, 27.9)}
pytestmark = pytest.mark.skipif(not OUT.exists(), reason="sample plans not generated")


@pytest.mark.parametrize("name", CASES)
def test_pdf_vector_areas(name):
    scale, _, area = CASES[name]
    dxf, rep = convert.to_dxf((OUT / f"{name}.pdf").read_bytes(), "application/pdf", scale=scale)
    assert rep.method == "pdf-vector"
    got = sum(p.net_area for p in dxf_plan.import_dxf(dxf).plans)
    assert got == pytest.approx(area, rel=0.1)


@pytest.mark.parametrize("name", CASES)
def test_raster_areas_with_known_length(name):
    _, longest, area = CASES[name]
    dxf, rep = convert.to_dxf((OUT / f"{name}.png").read_bytes(), "image/png", longest_m=longest)
    assert rep.method == "raster-trace" and rep.openings > 0
    assert "longest side" in rep.scale_source
    got = sum(p.net_area for p in dxf_plan.import_dxf(dxf).plans)
    assert got == pytest.approx(area, rel=0.1)


def test_implausible_scale_is_reported():
    _, rep = convert.to_dxf((OUT / "studio.pdf").read_bytes(), "application/pdf", scale=5)
    assert any("plausible" in w for w in rep.warnings)


def test_bad_input():
    with pytest.raises(EditError):
        convert.to_dxf(b"not an image", "image/png")
    with pytest.raises(EditError):
        convert.to_dxf(b"x", "image/gif")
    with pytest.raises(EditError):
        convert.to_dxf((OUT / "studio.png").read_bytes(), "image/png", longest_m=0.5)


def test_endpoint():
    c = TestClient(create_app(llm=object()))
    r = c.post("/v1/convert/dxf", files={"file": ("studio.pdf", (OUT / "studio.pdf").read_bytes(), "application/pdf")},
               data={"scale": "100"})
    assert r.status_code == 200 and r.json()["report"]["method"] == "pdf-vector"
    r = c.post("/v1/convert/dxf", files={"file": ("x.png", b"zzz", "image/png")})
    assert r.status_code == 422


def test_raster_reads_its_own_scale_and_room_names():
    """No scale given: the 'xx m²' labels on the picture set it, and the other text names the rooms."""
    dxf, rep = convert.to_dxf((OUT / "family_3br.png").read_bytes(), "image/png")
    assert "room areas" in rep.scale_source and rep.metres_across == pytest.approx(16.9, rel=0.05)
    assert rep.texts > 0
    plan = dxf_plan.import_dxf(dxf).plans[0]
    assert plan.net_area == pytest.approx(109.4, rel=0.1)


def test_raster_without_a_readable_scale_asks_for_one():
    with pytest.raises(EditError, match="longest side"):
        convert.to_dxf((OUT / "studio.png").read_bytes(), "image/png")


@pytest.mark.skipif(not (OUT / "user.png").exists(), reason="labelled apartment screenshot not present")
def test_labelled_screenshot_keeps_windows_fixtures_and_rooms():
    """A real labelled plan (RGBA PNG, coloured windows, furniture): 7 named rooms, windows apart from doors, furniture."""
    dxf, rep = convert.to_dxf((OUT / "user.png").read_bytes(), "image/png")
    plan = dxf_plan.import_dxf(dxf).plans[0]
    assert plan.room_count == 7
    assert {r.type for r in plan.model.rooms} >= {"bathroom", "bedroom", "kitchen", "dining", "living", "corridor"}
    kinds = [o.kind for o in plan.model.openings]
    assert kinds.count("window") >= 4 and kinds.count("door") >= 4
    assert len(plan.model.fixtures) >= 8
    assert plan.net_area == pytest.approx(92, rel=0.06)
