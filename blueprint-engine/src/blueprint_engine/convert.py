"""PDF / PNG / JPEG floor plan -> DXF (the 'original' a blueprint keeps). No LLM: geometry only.

  * PDF with vector content: the page's paths are read exactly (pdfium). Dark filled polygons are wall solids
    (layer A-WALL), curved strokes are door swings (A-DOOR), other strokes go to LINES, text becomes TEXT.
  * Raster (PNG/JPEG, or a scanned PDF page): classical image processing (OpenCV). Thick dark strokes are walls,
    their outlines become closed polylines; door-sized gaps between wall ends get a door mark. Thin strokes,
    hatching and text are NOT read (no OCR): they are listed as skipped.

The result is a normal DXF in metres, then read by `dxf_plan.import_dxf` like any CAD file, so the checks, the
viewer and the edit/patch path are the same. The scale is the one thing a picture cannot tell us: it comes from
`longest_m` (the real length of the drawing's longest side), else `scale` (paper scale 1:N) and the image DPI.
"""

from __future__ import annotations

import io
import math
import re

import ezdxf
import numpy as np
from pydantic import BaseModel, Field

from .edits import EditError

MAX_PIXELS = 24_000_000
DEFAULT_SCALE = 100
DEFAULT_DPI = 150
PT_MM = 25.4 / 72
DOOR_MIN, DOOR_MAX = 0.55, 3.2  # m: a gap between two wall ends this wide is an opening


class ConvertReport(BaseModel):
    method: str  # "pdf-vector" | "raster-trace"
    scale_source: str  # how metres per source unit was decided
    metres_across: float  # the drawing's longest side, m
    walls: int = 0  # wall solids / outlines written
    openings: int = 0
    texts: int = 0
    skipped: list[str] = Field(default_factory=list)
    warnings: list[str] = Field(default_factory=list)


class _Geo:
    """Drawing content in source units (y up). Closed wall polygons, loose lines, door marks, texts."""

    def __init__(self) -> None:
        self.walls: list[list[tuple[float, float]]] = []
        self.lines: list[list[tuple[float, float]]] = []
        self.arcs: list[list[tuple[float, float]]] = []
        self.doors: list[tuple[tuple[float, float], tuple[float, float], str]] = []  # a, b, door|window
        self.door_t: list[float] = []  # wall thickness at each gap (px), same order
        self.texts: list[tuple[str, float, float, float]] = []  # text, x, y, height
        self.fixtures: list[tuple[str, float, float, float, float]] = []  # kind, x, y, width, depth (px, y up)
        self.skipped: dict[str, int] = {}
        self.ink = None  # raster only: the masks the later steps read
        self.wall0 = None
        self.closed = None
        self.h = 0
        self.warnings: list[str] = []

    def extent(self) -> float:
        pts = [p for poly in (self.walls or self.lines) for p in poly]
        if not pts:
            return 0.0
        xs, ys = [p[0] for p in pts], [p[1] for p in pts]
        return max(max(xs) - min(xs), max(ys) - min(ys))


# ---------------------------------------------------------------- PDF


def _main_cluster(polys, gap: float):
    """The biggest group of polygons whose bounding boxes lie within `gap` of each other: the building, not the
    title block, legend or north arrow."""
    if len(polys) < 2:
        return polys
    boxes = [(min(p[0] for p in q), min(p[1] for p in q), max(p[0] for p in q), max(p[1] for p in q)) for q in polys]
    parent = list(range(len(polys)))

    def find(i):
        while parent[i] != i:
            parent[i] = parent[parent[i]]
            i = parent[i]
        return i

    order = sorted(range(len(boxes)), key=lambda i: boxes[i][0])
    for a, i in enumerate(order):
        for j in order[a + 1:]:
            if boxes[j][0] > boxes[i][2] + gap:
                break
            if boxes[j][1] <= boxes[i][3] + gap and boxes[i][1] <= boxes[j][3] + gap:
                parent[find(i)] = find(j)
    weight: dict[int, float] = {}
    for i, q in enumerate(polys):
        weight[find(i)] = weight.get(find(i), 0.0) + _area(q)
    best = max(weight, key=weight.get)
    return [q for i, q in enumerate(polys) if find(i) == best]


def _bezier(p0, p1, p2, p3, n: int = 8) -> list[tuple[float, float]]:
    out = []
    for i in range(1, n + 1):
        t = i / n
        u = 1 - t
        out.append((u**3 * p0[0] + 3 * u * u * t * p1[0] + 3 * u * t * t * p2[0] + t**3 * p3[0],
                    u**3 * p0[1] + 3 * u * u * t * p1[1] + 3 * u * t * t * p2[1] + t**3 * p3[1]))
    return out


def _area(poly) -> float:
    return abs(sum(a[0] * b[1] - b[0] * a[1] for a, b in zip(poly, poly[1:] + poly[:1]))) / 2


def _pdf_geo(data: bytes) -> tuple[_Geo, float, float]:
    """(geometry, page width pt, height pt) from the first page. Raises when the page has no vector paths."""
    import ctypes as C

    import pypdfium2 as pdfium
    import pypdfium2.raw as raw

    try:
        pdf = pdfium.PdfDocument(data)
    except Exception as e:
        raise EditError(f"cannot read the PDF: {e}") from e
    if len(pdf) == 0:
        raise EditError("the PDF has no pages")
    page = pdf[0]
    pw, ph = page.get_size()
    geo = _Geo()
    if len(pdf) > 1:
        geo.warnings.append(f"the PDF has {len(pdf)} pages: only page 1 was read")
    paths = 0
    for o in page.get_objects(max_depth=4):
        if o.type != raw.FPDF_PAGEOBJ_PATH:
            continue
        paths += 1
        l, b, r, t = o.get_bounds()
        if (r - l) >= 0.95 * pw and (t - b) >= 0.95 * ph:  # the sheet border / background
            continue
        fill, stroke = C.c_int(), C.c_int()
        raw.FPDFPath_GetDrawMode(o.raw, C.byref(fill), C.byref(stroke))
        cr = [C.c_uint() for _ in range(4)]
        raw.FPDFPageObj_GetFillColor(o.raw, *map(C.byref, cr))
        lum = (0.299 * cr[0].value + 0.587 * cr[1].value + 0.114 * cr[2].value) / 255
        m = o.get_matrix()
        subs: list[tuple[list[tuple[float, float]], bool]] = []
        cur: list[tuple[float, float]] = []
        curved = False
        pend: list[tuple[float, float]] = []
        for i in range(raw.FPDFPath_CountSegments(o.raw)):
            seg = raw.FPDFPath_GetPathSegment(o.raw, i)
            x, y = C.c_float(), C.c_float()
            raw.FPDFPathSegment_GetPoint(seg, x, y)
            pt = (m.a * x.value + m.c * y.value + m.e, m.b * x.value + m.d * y.value + m.f)
            kind = raw.FPDFPathSegment_GetType(seg)
            if kind == raw.FPDF_SEGMENT_MOVETO:
                if len(cur) > 1:
                    subs.append((cur, False))
                cur, pend = [pt], []
            elif kind == raw.FPDF_SEGMENT_LINETO:
                cur.append(pt)
            else:  # BEZIERTO comes in threes
                pend.append(pt)
                if len(pend) == 3 and cur:
                    cur.extend(_bezier(cur[-1], *pend))
                    curved, pend = True, []
            if raw.FPDFPathSegment_GetClose(seg) and len(cur) > 2:
                subs.append((cur, True))
                cur = []
        if len(cur) > 1:
            subs.append((cur, False))
        for pts, closed in subs:
            if fill.value and len(pts) > 2 and lum < 0.4 and _area(pts) > 1.0:
                geo.walls.append(pts)
            elif stroke.value or not fill.value:
                (geo.arcs if curved else geo.lines).append(pts)
            elif fill.value and lum >= 0.4:
                geo.skipped["white or light fills"] = geo.skipped.get("white or light fills", 0) + 1
    geo.walls = _main_cluster(geo.walls, 0.03 * max(pw, ph))
    if paths == 0:
        raise EditError("the PDF has no vector drawing (it is a scan): it is read as an image instead")
    tp = page.get_textpage()
    for i in range(tp.count_rects()):
        x0, y0, x1, y1 = tp.get_rect(i)
        txt = tp.get_text_bounded(x0, y0, x1, y1).strip()
        if txt:
            geo.texts.append((txt, x0, y0, max(y1 - y0, 1.0)))
    return geo, pw, ph


# ---------------------------------------------------------------- raster


def _decode(data: bytes):
    """(luminance, darkest-channel, dpi). Both grayscale, background white."""
    import cv2
    from PIL import Image

    dpi = None
    try:
        im = Image.open(io.BytesIO(data))
        if im.width * im.height > MAX_PIXELS:
            raise EditError("the image is larger than 24 megapixels")
        d = im.info.get("dpi")
        if d and d[0] and 30 <= float(d[0]) <= 1200:
            dpi = float(d[0])
    except EditError:
        raise
    except Exception as e:
        raise EditError(f"cannot read the image: {e}") from e
    img = cv2.imdecode(np.frombuffer(data, np.uint8), cv2.IMREAD_UNCHANGED)
    if img is None:
        raise EditError("cannot read the image")
    if img.dtype != np.uint8:
        img = cv2.convertScaleAbs(img, alpha=255.0 / max(1, int(img.max())))
    if img.ndim == 3:
        if img.shape[2] == 4:  # transparent background: composite on white
            a = img[..., 3:4].astype(np.float32) / 255.0
            img = (img[..., :3] * a + 255 * (1 - a)).astype(np.uint8)
        # walls and text come from the luminance (black strokes); `fine` from the darkest channel, so coloured
        # strokes (blue window frames, cyan fixtures) count as ink for windows and furniture only
        fine = img[..., :3].min(axis=2)
        img = cv2.cvtColor(img[..., :3], cv2.COLOR_BGR2GRAY)
        return img, fine, dpi
    return img, img, dpi


def _raster_geo(gray, m_per_px: float, fine=None) -> _Geo:
    """Walls = thick dark strokes. All sizes are derived from `m_per_px`, so the scale must be known first."""
    import cv2

    geo = _Geo()
    h, w = gray.shape
    _, ink = cv2.threshold(gray, 0, 255, cv2.THRESH_BINARY_INV | cv2.THRESH_OTSU)
    ink_px = int(np.count_nonzero(ink))
    if ink_px == 0:
        raise EditError("the image has no dark drawing on it")
    r = max(1, int(round(0.04 / m_per_px)))  # half of the thinnest wall we accept (8 cm)
    k = cv2.getStructuringElement(cv2.MORPH_RECT, (2 * r + 1, 2 * r + 1))
    walls = cv2.morphologyEx(ink, cv2.MORPH_OPEN, k)
    min_area = (0.3 / m_per_px) ** 2 * 0.1  # a wall piece must be at least ~0.3 m x 0.3 m (a jamb beside a door)
    n, lab, stats, _ = cv2.connectedComponentsWithStats(walls, connectivity=8)
    keep = np.zeros_like(walls)
    dropped = 0
    for i in range(1, n):
        if stats[i, cv2.CC_STAT_AREA] >= min_area:
            keep[lab == i] = 255
        else:
            dropped += 1
    if np.count_nonzero(keep) < 0.002 * h * w:
        # walls drawn as outlines (double lines): hand the ink to the importer as loose linework
        geo.warnings.append("no thick wall strokes found: the walls look like double lines, so all dark strokes are "
                            "kept as one layer; check the result")
        keep = ink
        mode_lines = True
    else:
        mode_lines = False
    eps = max(1.0, 0.03 / m_per_px)

    def contours(mask) -> list[list[tuple[float, float]]]:
        out = []
        cnts, _ = cv2.findContours(mask, cv2.RETR_LIST, cv2.CHAIN_APPROX_SIMPLE)
        for c in cnts:
            if cv2.contourArea(c) < 4 * r * r and len(c) < 8:
                continue
            a = cv2.approxPolyDP(c, eps, True)[:, 0, :]
            if len(a) >= 3:
                out.append(_square_up([(float(x), float(h - y)) for x, y in a], 2 * eps))
        return out

    if mode_lines:
        geo.lines = contours(keep)
    else:
        geo.walls = contours(keep)
        # a plan drawn with solid walls has the doorways and windows open: find them, close the wall mask across
        # them (so rooms are closed, as in a CAD plan whose walls run through the openings) and mark each one
        _door_gaps(geo, m_per_px)
        fine_src = gray if fine is None else fine
        geo.ink = cv2.threshold(fine_src, 200, 255, cv2.THRESH_BINARY_INV)[1]  # light grey furniture lines count
        geo.wall0, geo.h = keep.copy(), h
        if geo.doors:
            for (a, b, _), t in zip(geo.doors, geo.door_t):  # a bar as thick as the wall, so the wall runs on unbroken
                cv2.line(keep, (int(a[0]), int(h - a[1])), (int(b[0]), int(h - b[1])), 255, max(2, int(t)))
            geo.walls = contours(keep)
        geo.closed = keep
    if not mode_lines:
        if dropped:
            geo.skipped["small dark blobs (text, fixtures)"] = dropped
    return geo


def _square_up(poly: list[tuple[float, float]], tol: float) -> list[tuple[float, float]]:
    """Plans are mostly orthogonal: an edge within ~6 degrees of horizontal or vertical is made exactly so, and
    the tiny steps a ragged outline leaves (shorter than `tol`) are dropped. Other angles are kept."""
    pts = [list(p) for p in poly]
    n = len(pts)
    for i in range(n):
        a, b = pts[i], pts[(i + 1) % n]
        dx, dy = b[0] - a[0], b[1] - a[1]
        if abs(dx) < max(0.1 * abs(dy), min(1.5, 0.5 * abs(dy))):  # a skew of a pixel or two on a short edge is noise
            b[0] = a[0]
        elif abs(dy) < max(0.1 * abs(dx), min(1.5, 0.5 * abs(dx))):
            b[1] = a[1]
    out: list[tuple[float, float]] = []
    for p in pts:
        if not out or math.dist(out[-1], p) > 0.5:
            out.append((p[0], p[1]))
    # drop collinear middle points
    keep = []
    m = len(out)
    for i in range(m):
        a, b, c = out[i - 1], out[i], out[(i + 1) % m]
        if (a[0] == b[0] == c[0]) or (a[1] == b[1] == c[1]):
            continue
        keep.append(b)
    return keep if len(keep) >= 3 else out


def _door_gaps(geo: _Geo, m_per_px: float) -> None:
    """Door marks across gaps: two short wall end-caps that face each other DOOR_MIN..DOOR_MAX apart on one line."""
    from shapely.geometry import Point, Polygon

    shapes = [Polygon(q) for q in geo.walls]
    caps = []  # (midpoint, unit normal pointing out of the wall material, length)
    for pi, poly in enumerate(geo.walls):
        # the right-hand normal points out of a counter-clockwise ring; a ring inside another is a hole, whose
        # material is outside it, so the sense flips
        ccw = sum(a[0] * b[1] - b[0] * a[1] for a, b in zip(poly, poly[1:] + poly[:1])) > 0
        is_hole = any(j != pi and shapes[j].contains(Point(poly[0])) for j in range(len(shapes)))
        sign = 1.0 if ccw != is_hole else -1.0
        n = len(poly)
        for i in range(n):
            a, b = poly[i], poly[(i + 1) % n]
            ln = math.dist(a, b)
            if not 0.05 <= ln * m_per_px <= 0.45:
                continue
            mid = ((a[0] + b[0]) / 2, (a[1] + b[1]) / 2)
            nx, ny = sign * (b[1] - a[1]) / ln, -sign * (b[0] - a[0]) / ln
            # polygons from findContours are not consistently wound: test the side by probing is skipped,
            # so keep both normals and let the pair test decide
            caps.append((mid, (nx, ny), ln))
    used: set[int] = set()
    for i, (m1, n1, l1) in enumerate(caps):
        if i in used:
            continue
        best = None
        for j in range(i + 1, len(caps)):
            if j in used:
                continue
            m2, n2, l2 = caps[j]
            if abs(n1[0] * n2[0] + n1[1] * n2[1]) < 0.97 or abs(l1 - l2) > 0.4 * max(l1, l2):
                continue
            dx, dy = m2[0] - m1[0], m2[1] - m1[1]
            d = math.hypot(dx, dy)
            if d == 0 or dx * n1[0] + dy * n1[1] <= 0 or dx * n2[0] + dy * n2[1] >= 0:  # the two ends must face each other
                continue
            if not DOOR_MIN <= d * m_per_px <= DOOR_MAX:
                continue
            if abs(dx * n1[1] - dy * n1[0]) > 0.2 * max(l1, l2):  # not on one line: lateral offset
                continue
            if abs((dx * n1[0] + dy * n1[1]) / d) < 0.97:  # the gap axis must follow the cap normal
                continue
            if best is None or d < best[0]:
                best = (d, j)
        if best:
            used.update((i, best[1]))
            m2 = caps[best[1]][0]
            mx, my = (m1[0] + m2[0]) / 2, (m1[1] + m2[1]) / 2
            reach = math.dist(m1, m2) * 0.6
            swing = any(abs(p[0] - mx) < reach and abs(p[1] - my) < reach for arc in geo.arcs for p in arc[:: max(1, len(arc) // 4)])
            geo.doors.append((m1, m2, "door" if (swing or not geo.arcs) else "window"))
            geo.door_t.append((l1 + caps[best[1]][2]) / 2)


# ---------------------------------------------------------------- OCR and scale

_OCR = None
AREA_RE = re.compile(r"^\s*(\d{1,3}(?:[.,]\d{1,2})?)\s*m\s*[²2*^]?\s*$", re.I)
NUM_RE = re.compile(r"^\s*(\d{1,3}(?:[.,]\d{1,2})?)\s*(?:m|cm)?\s*$", re.I)


def _ocr(gray) -> list[tuple[str, float, float, float]]:
    """(text, x, y, height) in pixels (y down) for every text the OCR reads; [] when OCR is unavailable."""
    global _OCR
    try:
        if _OCR is None:
            from rapidocr_onnxruntime import RapidOCR

            _OCR = RapidOCR()
        res, _ = _OCR(gray)
    except Exception:
        return []
    out = []
    for box, txt, conf in res or []:
        if float(conf) < 0.6 or not txt.strip():
            continue
        xs, ys = [p[0] for p in box], [p[1] for p in box]
        out.append((txt.strip(), (min(xs) + max(xs)) / 2, (min(ys) + max(ys)) / 2, max(ys) - min(ys)))
    return out


def _faces(geo: _Geo):
    """Closed faces between the traced walls (px, y up): the rooms, then the wall pieces themselves."""
    from shapely.geometry import LineString
    from shapely.ops import polygonize, unary_union

    lines = [LineString(p + [p[0]]) for p in geo.walls]
    return [f for f in polygonize(unary_union(lines)) if f.area > 50]


def _scale_from_labels(geo: _Geo, texts: list[tuple[str, float, float, float]], h: int, longest_px: float
                       ) -> tuple[float, str] | None:
    """Metres per pixel from what the drawing says about itself: first the 'xx.x m²' labels against the rooms
    that carry them, then the overall dimension numbers against the wall extent."""
    from shapely.geometry import Point

    faces = _faces(geo)
    ratios = []
    for txt, x, y, _ in texts:
        m = AREA_RE.match(txt)
        if not m or "m" not in txt.lower():
            continue
        area = float(m.group(1).replace(",", "."))
        inside = [f for f in faces if f.contains(Point(x, h - y))]
        if area >= 2 and inside:
            ratios.append(math.sqrt(area / min(f.area for f in inside)))
    if len(ratios) >= 2:
        ratios.sort()
        med = ratios[len(ratios) // 2]
        agree = [r for r in ratios if abs(r - med) / med < 0.1]  # a label beside a fixture can misread: outvote it
        if len(agree) >= 2 and len(agree) * 2 > len(ratios):
            return sum(agree) / len(agree), f"the room areas written on the plan ({len(agree)} of {len(ratios)} labels agree)"
    nums = sorted((float(NUM_RE.match(t).group(1).replace(",", ".")) for t, *_ in texts
                   if NUM_RE.match(t) and "m" not in t.lower() and 1.0 <= float(NUM_RE.match(t).group(1).replace(",", ".")) <= 100), reverse=True)
    if nums and nums[0] >= 5.0:  # a smaller number is a room dimension, not the overall one
        return nums[0] / longest_px, f"the largest dimension written on the plan ({nums[0]:g} m)"
    gaps = sorted(math.dist(a, b) for a, b, _ in geo.doors)
    if len(gaps) >= 3:  # last resort: the narrower openings are doors, and a door is about 0.9 m
        return 0.9 / gaps[len(gaps) // 4], "estimated from door width (openings assumed 0.9 m): please check"
    return None


# ---------------------------------------------------------------- windows, doors and fixtures (pictures)


def _classify_openings(geo: _Geo, k: float) -> None:
    """Door or window for each gap: a door swing leaves thin strokes beside the gap; a window leaves none and sits
    in the outside wall. Everything else is a doorway."""
    import cv2

    if geo.ink is None or not geo.doors:
        return
    h = geo.h
    ink, wall0, closed = geo.ink, geo.wall0, geo.closed
    free = cv2.bitwise_not(closed)
    n, lab = cv2.connectedComponents(free, connectivity=4)
    border = set(np.unique(np.concatenate([lab[0], lab[-1], lab[:, 0], lab[:, -1]]))) - {0}
    outside = np.isin(lab, list(border)) & (free > 0)
    thin = cv2.bitwise_and(ink, cv2.bitwise_not(cv2.dilate(wall0, np.ones((5, 5), np.uint8))))
    out = []
    for a, b, _ in geo.doors:
        ax, ay, bx, by = a[0], h - a[1], b[0], h - b[1]
        cx, cy = (ax + bx) / 2, (ay + by) / 2
        w = math.hypot(bx - ax, by - ay)
        nx, ny = -(by - ay) / w, (bx - ax) / w
        step = 0.35 / k  # px for 0.35 m
        def at(mask, x, y):
            xi, yi = int(round(x)), int(round(y))
            return 0 <= yi < mask.shape[0] and 0 <= xi < mask.shape[1] and bool(mask[yi, xi])
        ext = any(not (0 <= int(cy + sg * ny * step) < outside.shape[0] and 0 <= int(cx + sg * nx * step) < outside.shape[1])
                  or at(outside, cx + sg * nx * step, cy + sg * ny * step) for sg in (-1, 1))
        # a door leaf is a straight stroke from the hinge (a gap end) square to the wall, about the gap wide;
        # furniture, dimension lines and text do not make such a stroke
        door = False
        ink2 = cv2.dilate(thin, np.ones((3, 3), np.uint8))
        for ex, ey in ((ax, ay), (bx, by)):
            for sg in (-1, 1):
                hits = 0
                samples = [0.3 + 0.65 * i / 14 for i in range(15)]
                for f in samples:
                    hits += at(ink2, ex + sg * nx * w * f, ey + sg * ny * w * f)
                if hits >= 0.8 * len(samples):
                    door = True
        out.append((a, b, "door" if (door or not ext) else "window"))
    geo.doors = out


def _fixture_candidates(geo: _Geo, texts, k: float):
    """Furniture and sanitary outlines: thin strokes inside the building, away from text, doors and walls."""
    import cv2

    ink, wall0, closed, h = geo.ink, geo.wall0, geo.closed, geo.h
    px = lambda m: int(round(m / k))
    thin = cv2.bitwise_and(ink, cv2.bitwise_not(cv2.dilate(wall0, np.ones((5, 5), np.uint8))))
    free = cv2.bitwise_not(closed)
    n, lab = cv2.connectedComponents(free, connectivity=4)
    border = set(np.unique(np.concatenate([lab[0], lab[-1], lab[:, 0], lab[:, -1]]))) - {0}
    inside = (~np.isin(lab, list(border))) & (free > 0)
    thin[~inside] = 0
    for txt, x, y, hgt in texts:  # text boxes (names, areas, dimensions): just the glyphs, not the furniture around
        hh, hw = int(hgt * 0.6) + 2, int(len(txt) * hgt * 0.36) + 4
        thin[max(0, int(y) - hh): int(y) + hh, max(0, int(x) - hw): int(x) + hw] = 0
    for (a, b, kind), t in zip(geo.doors, geo.door_t):
        cx, cy = (a[0] + b[0]) / 2, h - (a[1] + b[1]) / 2
        if kind == "window":  # the frame lines in the gap must not join the furniture beside it
            cv2.line(thin, (int(a[0]), int(h - a[1])), (int(b[0]), int(h - b[1])), 0, int(t * 1.6) + 2)
        else:  # door leaf and swing arc
            cv2.circle(thin, (int(cx), int(cy)), int(math.hypot(b[0] - a[0], b[1] - a[1]) * 1.15), 0, -1)
    joined = cv2.dilate(thin, np.ones((max(3, px(0.04)),) * 2, np.uint8))
    nn, lb, st, _ = cv2.connectedComponentsWithStats(joined, connectivity=8)
    for i in range(1, nn):
        x, y, w, hh, _ = st[i]
        yield x, y, w, hh, (lb[y:y + hh, x:x + w] == i), thin[y:y + hh, x:x + w]


def _detect_fixtures(geo: _Geo, texts, k: float) -> None:
    """Fixtures by size, proportion and the room they stand in (named by the OCR text). A rule set, not a model:
    only the kinds below are found, and a piece that matches none is left out."""
    import cv2

    from . import dxf_plan

    if geo.ink is None or geo.closed is None:
        return
    h = geo.h
    faces = _faces(geo)
    from shapely.geometry import Point

    def room_of(x, y):
        inside = [f for f in faces if f.contains(Point(x, h - y))]
        if not inside:
            return "other"
        f = min(inside, key=lambda g: g.area)
        names = [t.replace("_", " ") for t, tx, ty, _ in texts if f.contains(Point(tx, h - ty)) and not AREA_RE.match(t) and not NUM_RE.match(t)]
        return dxf_plan._room_type(names)

    # pieces of one piece of furniture (outline, pillows, blanket) lie within a few cm of each other: group them
    boxes = [[x, y, x + w, y + hh, thin] for x, y, w, hh, _, thin in _fixture_candidates(geo, texts, k)]
    gap = 0.05 / k
    merged = True
    while merged:
        merged = False
        for i in range(len(boxes)):
            for j in range(i + 1, len(boxes)):
                a_, b_ = boxes[i], boxes[j]
                if a_[0] - gap <= b_[2] and b_[0] - gap <= a_[2] and a_[1] - gap <= b_[3] and b_[1] - gap <= a_[3]:
                    boxes[i] = [min(a_[0], b_[0]), min(a_[1], b_[1]), max(a_[2], b_[2]), max(a_[3], b_[3]), a_[4]]
                    del boxes[j]
                    merged = True
                    break
            if merged:
                break
    found = 0
    for x0, y0, x1, y1, thin in boxes:
        x, y, w, hh = x0, y0, x1 - x0, y1 - y0
        thin = thin[:0]  # the stove/fridge test below reads the pixels of the merged box instead
        thin = geo.ink[y0:y1, x0:x1]
        wm, dm = w * k, hh * k
        long_, short = max(wm, dm), min(wm, dm)
        if short < 0.25 or long_ < 0.3 or long_ > 3.0:
            continue
        cx, cy = x + w / 2, y + hh / 2
        room = room_of(cx, cy)
        holes = 0
        if short <= 1.0:
            cnts, hier = cv2.findContours(np.pad(thin, 1), cv2.RETR_CCOMP, cv2.CHAIN_APPROX_SIMPLE)
            holes = sum(1 for hr in (hier[0] if hier is not None else []) if hr[3] >= 0)
        kind = None
        if room == "bedroom":
            if 1.1 <= short <= 2.4 and 1.5 <= long_ <= 2.7:
                kind = "bed"
            elif short <= 0.8 and long_ >= 0.8:
                kind = "wardrobe"
        elif room == "bathroom":
            if 0.75 <= short and long_ <= 1.3:
                kind = "shower"
            elif short >= 0.65 and long_ >= 1.5:
                kind = "bathtub"
            elif long_ <= 0.8:
                kind = "toilet" if long_ / short > 1.25 else "basin"
        elif room == "kitchen":
            if short <= 0.8 and long_ >= 1.2:
                kind = "kitchen_counter"
            elif 0.5 <= short and long_ <= 0.9:
                kind = "stove" if holes >= 3 else "fridge"
            elif long_ <= 0.6:
                kind = "sink"
        elif room == "dining":
            if long_ >= 1.4 and 0.75 <= short <= 1.3:
                kind = "dining_table"
        elif room == "living":
            if long_ >= 1.4 and 0.6 <= short <= 1.1:
                kind = "sofa"
            elif short <= 0.5 and long_ >= 1.0:
                kind = "tv_unit"
        elif room == "study":
            if long_ >= 1.0 and 0.5 <= short <= 0.9:
                kind = "desk"
        elif room in ("storage", "laundry"):
            kind = "shelves" if short <= 0.6 and long_ >= 0.8 else ("washer" if 0.5 <= short and long_ <= 0.8 else None)
        if kind:
            geo.fixtures.append((kind, cx, h - cy, w, hh))
            found += 1
    if not found:
        geo.warnings.append("no furniture or sanitary fixtures were recognised (only labelled rooms are searched)")


# ---------------------------------------------------------------- DXF writer


def _dxf(geo: _Geo, k: float, flip_x: float = 0.0) -> bytes:
    """k: metres per source unit. Output is in metres with $INSUNITS = 6."""
    doc = ezdxf.new("R2010", setup=True, units=ezdxf.units.M)
    for name, color in (("A-WALL", 7), ("A-DOOR", 3), ("A-GLAZ", 4), ("LINES", 8), ("A-ANNO-TEXT", 2)):
        doc.layers.add(name, color=color)
    msp = doc.modelspace()
    pts = [p for poly in (geo.walls + geo.lines + geo.arcs) for p in poly]
    ox = min((p[0] for p in pts), default=0.0)
    oy = min((p[1] for p in pts), default=0.0)

    def m(p):
        return ((p[0] - ox) * k, (p[1] - oy) * k)

    for poly in geo.walls:
        msp.add_lwpolyline([m(p) for p in poly], close=True, dxfattribs={"layer": "A-WALL"})
    for poly in geo.lines:
        msp.add_lwpolyline([m(p) for p in poly], dxfattribs={"layer": "LINES"})
    for poly in geo.arcs:
        msp.add_lwpolyline([m(p) for p in poly], dxfattribs={"layer": "A-DOOR"})
    from .draw import FIXTURE_LAYERS

    for i, (kind, x, y, w, d) in enumerate(geo.fixtures):
        name = f"{kind}_{i + 1}"
        blk = doc.blocks.new(name=name)
        hw, hd = w * k / 2, d * k / 2
        blk.add_lwpolyline([(-hw, -hd), (hw, -hd), (hw, hd), (-hw, hd)], close=True)
        layer = FIXTURE_LAYERS.get(kind, "A-FURN")
        if layer not in doc.layers:
            doc.layers.add(layer, color=5)
        msp.add_blockref(name, m((x, y)), dxfattribs={"layer": layer})
    for a, b, kind in geo.doors:
        msp.add_line(m(a), m(b), dxfattribs={"layer": "A-DOOR" if kind == "door" else "A-GLAZ"})
    for txt, x, y, hgt in geo.texts:
        t = msp.add_text(txt, height=max(0.1, hgt * k), dxfattribs={"layer": "A-ANNO-TEXT"})
        t.dxf.insert = m((x, y))
    out = io.StringIO()
    doc.write(out)
    return out.getvalue().encode("utf-8")


# ---------------------------------------------------------------- entry


def to_dxf(data: bytes, mime: str, *, scale: int | None = None, dpi: float | None = None,
           longest_m: float | None = None) -> tuple[bytes, ConvertReport]:
    """Convert an uploaded PDF/PNG/JPEG into DXF bytes. `longest_m` wins over `scale`."""
    if scale is not None and not 1 <= scale <= 5000:
        raise EditError("scale must be 1:1 to 1:5000")
    if longest_m is not None and not 1.0 <= longest_m <= 500.0:
        raise EditError("the drawing's longest side must be 1 to 500 m")
    method = "raster-trace"
    geo: _Geo | None = None
    mm_per_unit = 0.0
    px_dpi = dpi
    if mime == "application/pdf":
        try:
            geo, pw, ph = _pdf_geo(data)
            method = "pdf-vector"
            mm_per_unit = PT_MM
        except EditError as e:
            if "no vector drawing" not in str(e):
                raise
            import pypdfium2 as pdfium

            img = pdfium.PdfDocument(data)[0].render(scale=300 / 72).to_pil().convert("L")
            gray = np.array(img)
            fine = gray
            px_dpi = px_dpi or 300.0
            geo = None
    elif mime in ("image/png", "image/jpeg"):
        gray, fine, meta_dpi = _decode(data)
        px_dpi = px_dpi or meta_dpi
    else:
        raise EditError("upload a PNG, JPG or PDF")

    if method == "pdf-vector":
        assert geo is not None
        if geo.extent() <= 0:
            raise EditError("the PDF has no wall-sized drawing on page 1")
        k, how = _scale(geo.extent() * mm_per_unit / 1000, mm_per_unit / 1000, scale, longest_m, None, vector=True)
    else:
        # every size threshold depends on the scale, so settle a starting scale first: the image's own DPI and the
        # stated paper scale if there are any, else the prior that a plan is about 12 m across
        h_px = gray.shape[0]
        ext_px = float(max(gray.shape))
        if scale or px_dpi:
            nominal = (25.4 / (px_dpi or DEFAULT_DPI)) / 1000 * (scale or DEFAULT_SCALE)
        else:
            nominal = 12.0 / ext_px
        texts = _ocr(gray)
        geo = _raster_geo(gray, nominal, fine)
        if longest_m:
            k = longest_m / max(geo.extent(), 1.0)
            how = f"you gave the longest side as {longest_m:g} m"
        elif scale:
            k, how = _scale(ext_px * nominal, nominal, scale, None, px_dpi, vector=False)
        else:
            # the labels need closed rooms, which need the right scale: refine until the scale stops moving
            k, how, cur, found = nominal, "", nominal, False
            for _ in range(3):
                est = _scale_from_labels(geo, texts, h_px, geo.extent())
                if est is None:
                    break
                k, how, found = est[0], est[1], True
                if abs(k - cur) / cur < 0.05:
                    break
                geo, cur = _raster_geo(gray, k, fine), k
            if not found:
                raise EditError("the scale of this picture could not be read from it (no room areas or overall "
                                "dimension recognised): enter the real length of the drawing's longest side")
        if longest_m and abs(k - nominal) / nominal > 0.15:
            geo = _raster_geo(gray, k, fine)
            k = longest_m / max(geo.extent(), 1.0)
        # room names: the OCR text that is not a number or an area
        for txt, x, y, hgt in texts:
            if AREA_RE.match(txt) or NUM_RE.match(txt):
                continue
            geo.texts.append((re.sub(r"[_]+", " ", txt), x - 0.0, h_px - y, hgt))
        if not texts:
            geo.warnings.append("no text could be read from the picture, so rooms are named Room n")
    if method == "pdf-vector":
        small = [w for w in geo.walls if _area(w) * k * k < 0.15]
        if small:  # glyphs drawn as outlines, arrowheads, symbols
            geo.walls = [w for w in geo.walls if _area(w) * k * k >= 0.15]
            geo.skipped["small filled shapes (text drawn as outlines, symbols)"] = len(small)
    if method == "pdf-vector" and geo.walls:
        _door_gaps(geo, k)
    if method == "raster-trace":
        _classify_openings(geo, k)
        _detect_fixtures(geo, texts, k)
    metres = geo.extent() * k
    rep = ConvertReport(method=method, scale_source=how, metres_across=round(metres, 2), walls=len(geo.walls) or len(geo.lines),
                        openings=len(geo.doors), texts=len(geo.texts),
                        skipped=[f"{n} × {what}" for what, n in geo.skipped.items()], warnings=list(geo.warnings))
    if method == "raster-trace":
        rep.warnings.append("read from a picture: walls, openings (door or window), room names and common furniture are "
                            "recognised by rules, so check the result; other furniture, door swing direction and "
                            "fixtures touching a wall may be missing")
    if not 3.0 <= metres <= 80.0:
        rep.warnings.append(f"the drawing comes out {metres:.1f} m across, which is not a plausible plan: give the real "
                            "length of its longest side (or the right scale) and convert again")
    if not (geo.walls or geo.lines):
        raise EditError("no walls could be found in the file")
    return _dxf(geo, k), rep


def _scale(extent_m_nominal: float, unit_m: float, scale: int | None, longest_m: float | None, dpi: float | None,
           *, vector: bool) -> tuple[float, str]:
    """(metres per source unit, how)."""
    if longest_m:
        return longest_m / (extent_m_nominal / unit_m), f"you gave the longest side as {longest_m:g} m"
    denom = scale or DEFAULT_SCALE
    if vector:
        return unit_m * denom, f"paper scale 1:{denom}" + ("" if scale else " (assumed: not stated)")
    d = dpi or DEFAULT_DPI
    src = f"image resolution {d:g} dpi" if dpi else f"{DEFAULT_DPI} dpi (assumed: the image has none)"
    return (25.4 / d) / 1000 * denom, f"paper scale 1:{denom}{'' if scale else ' (assumed)'} at {src}"
