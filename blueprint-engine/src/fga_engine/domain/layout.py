"""Zoned slicing layout.

Local frame: the entrance (street) side is y=0 ("front"), x runs along the
front. The house is three horizontal bands:

    back band   (private: bedrooms, bathrooms)   touches the back wall
    corridor    (hall, CORRIDOR_WIDTH deep)      every room opens onto it
    front band  (public: entry, living, kitchen) touches the front wall

Every room therefore touches the hall (reachability by construction) and an
exterior wall (windows by construction). The search enumerates which band a
room is in, the room order inside each band and a few house widths; hard rules
(side constraints, minimum widths) filter candidates, soft rules
(adjacency, aspect ratio, zoning, stability vs. the previous model) score them.
The search is deterministic: same brief + same previous model = same layout.
"""

from __future__ import annotations

import itertools
import math
import random
from dataclasses import dataclass, field

from .schemas import (
    CORRIDOR_WIDTH,
    EXTERIOR_WALL,
    GRID,
    INTERIOR_WALL,
    PRIVATE_TYPES,
    Brief,
    BuildingModel,
    Room,
    Side,
)

GROSS_FACTOR = 1.10  # centerline area needed per m² of net area (walls eat the rest)
WALL_ALLOWANCE = EXTERIOR_WALL / 2 + INTERIOR_WALL / 2 + 0.01  # worst-case centerline -> net loss
MAX_PERMS = 4000
LocalSide = str  # "front" | "back" | "left" | "right"

_OPPOSITE: dict[Side, Side] = {"south": "north", "north": "south", "east": "west", "west": "east"}
# world side of local x=0 ("left") for each entrance side
_LEFT: dict[Side, Side] = {"south": "west", "north": "east", "east": "south", "west": "north"}

# implicit soft adjacencies (weaker than the brief's explicit ones)
_IMPLICIT_ADJ = [("kitchen", "dining"), ("dining", "living"), ("entry", "living")]


def snap(v: float) -> float:
    return round(round(v / GRID) * GRID, 3)


def local_side(entrance: Side, world: Side) -> LocalSide:
    if world == entrance:
        return "front"
    if world == _OPPOSITE[entrance]:
        return "back"
    return "left" if world == _LEFT[entrance] else "right"


def world_side(entrance: Side, local: LocalSide) -> Side:
    return {
        "front": entrance,
        "back": _OPPOSITE[entrance],
        "left": _LEFT[entrance],
        "right": _OPPOSITE[_LEFT[entrance]],
    }[local]


@dataclass(frozen=True)
class Frame:
    """Maps the local lot frame (front at y=0) to world coordinates."""

    entrance: Side
    lot_w: float  # world x size
    lot_d: float  # world y size

    @property
    def local_w(self) -> float:
        return self.lot_w if self.entrance in ("south", "north") else self.lot_d

    @property
    def local_d(self) -> float:
        return self.lot_d if self.entrance in ("south", "north") else self.lot_w

    def point(self, x: float, y: float) -> tuple[float, float]:
        e = self.entrance
        if e == "south":
            return (x, y)
        if e == "north":
            return (self.local_w - x, self.local_d - y)
        if e == "east":
            return (self.lot_w - y, x)
        return (y, self.lot_d - x)  # west

    def rect(self, r: tuple[float, float, float, float]) -> tuple[float, float, float, float]:
        ax, ay = self.point(r[0], r[1])
        bx, by = self.point(r[2], r[3])
        return (round(min(ax, bx), 3), round(min(ay, by), 3), round(max(ax, bx), 3), round(max(ay, by), 3))


@dataclass
class Layout:
    frame: Frame
    footprint: tuple[float, float, float, float]  # world
    rooms: dict[str, tuple[float, float, float, float]]  # world centerline rects, incl. "hall"
    front: list[str]
    back: list[str]
    score: float
    notes: list[str] = field(default_factory=list)


class LayoutError(ValueError):
    """The brief cannot be laid out (reported back as facts the LLM can act on)."""


def _gross(room: Room) -> float:
    return room.target_area * GROSS_FACTOR


def _default_band(room: Room, entrance: Side) -> str | None:
    """Band a room is forced into by its side constraint, else None."""
    if room.side is None:
        return None
    ls = local_side(entrance, room.side)
    if ls in ("front", "back"):
        return ls
    return None


def _zone(room: Room) -> str:
    return "back" if room.type in PRIVATE_TYPES else "front"


def _band_assignments(brief: Brief) -> list[dict[str, str]]:
    base: dict[str, str] = {}
    flexible: list[str] = []
    for r in brief.top_level():
        forced = _default_band(r, brief.entrance_side)
        base[r.id] = forced or _zone(r)
        if forced is None:
            flexible.append(r.id)
    options = [base]
    for rid in flexible:
        alt = dict(base)
        alt[rid] = "back" if base[rid] == "front" else "front"
        options.append(alt)
    return [o for o in options if any(v == "front" for v in o.values())]


def _orders(ids: list[str], seed_order: list[str] | None) -> list[tuple[str, ...]]:
    if len(ids) <= 6:
        return list(itertools.permutations(ids))
    rng = random.Random(len(ids))  # deterministic
    seen: set[tuple[str, ...]] = set()
    out: list[tuple[str, ...]] = []
    candidates = [tuple(ids)]
    if seed_order:
        candidates.append(tuple(i for i in seed_order if i in ids) + tuple(i for i in ids if i not in seed_order))
    for c in candidates:
        if c not in seen:
            seen.add(c)
            out.append(c)
    while len(out) < MAX_PERMS:
        p = ids[:]
        rng.shuffle(p)
        t = tuple(p)
        if t not in seen:
            seen.add(t)
            out.append(t)
    return out


def _adjacency_pairs(brief: Brief) -> tuple[set[frozenset[str]], set[frozenset[str]]]:
    # a nested room's adjacency requests count for its parent's block
    block = {r.id: r.parent or r.id for r in brief.rooms}
    explicit = {frozenset((block[r.id], block[a])) for r in brief.rooms for a in r.adjacent_to if block[r.id] != block[a]}
    by_type: dict[str, list[str]] = {}
    for r in brief.top_level():
        by_type.setdefault(r.type, []).append(r.id)
    implicit = set()
    for a, b in _IMPLICIT_ADJ:
        for x in by_type.get(a, []):
            for y in by_type.get(b, []):
                pair = frozenset((x, y))
                if pair not in explicit:
                    implicit.add(pair)
    return explicit, implicit


def _min_cut(room: Room) -> float:
    """Smallest centerline width that still gives the room its net minimum width."""
    return math.ceil((room.effective_min_width() + WALL_ALLOWANCE) / GRID - 1e-9) * GRID


@dataclass(frozen=True)
class Blocks:
    """Top-level rooms; a room holding nested rooms (ensuite, walk-in closet) is one block."""

    gross: dict[str, float]  # centerline area the block needs
    min_cut: dict[str, float]  # narrowest band width for the block
    min_depth: dict[str, float]  # shallowest band depth for the block
    children: dict[str, list[Room]]


def _blocks(brief: Brief) -> Blocks:
    gross: dict[str, float] = {}
    cut: dict[str, float] = {}
    depth: dict[str, float] = {}
    kids: dict[str, list[Room]] = {}
    for r in brief.top_level():
        ch = brief.children(r.id)
        kids[r.id] = ch
        gross[r.id] = _gross(r) + sum(_gross(c) for c in ch)
        cut[r.id] = _min_cut(r) + (max(_min_cut(c) for c in ch) if ch else 0.0)
        depth[r.id] = max(_min_cut(r), sum(_min_cut(c) for c in ch))
    return Blocks(gross, cut, depth, kids)


def stack_depths(children: list[Room], depth: float) -> list[float]:
    """Depth share of each stacked nested room: proportional to area, but never below its
    minimum width (a closet must not squeeze an ensuite)."""
    mins = [_min_cut(c) for c in children]
    gross = [_gross(c) for c in children]
    if sum(mins) > depth + 1e-6:
        return [depth * g / sum(gross) for g in gross]
    fixed: dict[int, float] = {}
    while True:
        free = [i for i in range(len(children)) if i not in fixed]
        free_d = depth - sum(fixed.values())
        free_a = sum(gross[i] for i in free)
        share = {i: free_d * gross[i] / free_a for i in free}
        under = [i for i in free if share[i] < mins[i]]
        if not under:
            return [fixed.get(i, share.get(i, 0.0)) for i in range(len(children))]
        for i in under:
            fixed[i] = mins[i]


def slice_width(children: list[Room], depth: float) -> float:
    """Width of the full-depth strip that nested rooms take off one end of their parent."""
    if not children:
        return 0.0
    hs = stack_depths(children, depth)
    # rooms held at their minimum depth are already big enough; the others set the strip width
    free = [(c, h) for c, h in zip(children, hs) if h > _min_cut(c) + 1e-6]
    w = max(_gross(c) / h for c, h in free) if free else sum(_gross(c) for c in children) / max(depth, 0.1)
    w = max(w, max(_min_cut(c) for c in children))
    return snap(w + GRID / 2)


def _allocate(order: tuple[str, ...], blocks: Blocks, width: float) -> list[float] | None:
    """Widths proportional to area, but never below a block's minimum; the rest is shared out."""
    mins = {rid: blocks.min_cut[rid] for rid in order}
    if sum(mins.values()) > width + 1e-6:
        return None
    fixed: dict[str, float] = {}
    while True:
        free_ids = [rid for rid in order if rid not in fixed]
        free_w = width - sum(fixed.values())
        free_a = sum(blocks.gross[rid] for rid in free_ids)
        prop = {rid: free_w * blocks.gross[rid] / free_a for rid in free_ids}
        under = [rid for rid in free_ids if prop[rid] < mins[rid]]
        if not under:
            return [fixed.get(rid, prop.get(rid, 0.0)) for rid in order]
        for rid in under:
            fixed[rid] = mins[rid]


@dataclass
class _BandResult:
    order: tuple[str, ...]
    cuts: list[tuple[str, float, float]]  # id, x0, x1 (local)
    score: float
    hard_ok: bool


def _place_band(
    brief: Brief,
    blocks: Blocks,
    ids: list[str],
    width: float,
    depth: float,
    y0: float,
    explicit: set[frozenset[str]],
    implicit: set[frozenset[str]],
    prev_centers: dict[str, tuple[float, float]],
    frame: Frame,
    origin: tuple[float, float],
    prev_order: list[str] | None,
) -> _BandResult:
    rooms = {r.id: r for r in brief.rooms}
    total = sum(blocks.gross[i] for i in ids)
    best: _BandResult | None = None
    for order in _orders(ids, prev_order):
        # hard: left/right side constraints
        hard_ok = True
        for idx, rid in enumerate(order):
            side = rooms[rid].side
            if side is None:
                continue
            ls = local_side(brief.entrance_side, side)
            if ls == "left" and idx != 0:
                hard_ok = False
            if ls == "right" and idx != len(order) - 1:
                hard_ok = False
        widths = _allocate(order, blocks, width)
        if widths is None:  # minimum widths alone exceed the band
            hard_ok = False
            widths = [width * blocks.gross[rid] / total for rid in order]
        cuts: list[tuple[str, float, float]] = []
        x = 0.0
        for idx, rid in enumerate(order):
            w = round(width - x, 3) if idx == len(order) - 1 else snap(widths[idx])
            cuts.append((rid, x, x + w))
            x += w
        score = 0.0
        for rid, a, b in cuts:
            w = b - a
            wp = w - slice_width(blocks.children[rid], depth)  # the parent after its nested rooms
            net_short = min(wp, depth) - WALL_ALLOWANCE
            if net_short + 1e-6 < rooms[rid].effective_min_width():
                hard_ok = False
                score += 50 * (rooms[rid].effective_min_width() - net_short)
            # area pushed onto a room by its minimum width shows up as target deviation
            dev = abs(w * depth - blocks.gross[rid]) / blocks.gross[rid]
            score += 4.0 * dev
            ratio = max(w, depth) / max(min(w, depth), 0.1)
            if ratio > 1.6:
                score += 2.0 * (ratio - 1.6) ** 2
        neighbours = {frozenset((order[i], order[i + 1])) for i in range(len(order) - 1)}
        for pair in explicit:
            if pair <= set(ids):
                score += -3.0 if pair in neighbours else 3.0
        for pair in implicit:
            if pair <= set(ids):
                score += -1.0 if pair in neighbours else 1.0
        for rid, a, b in cuts:
            if rid in prev_centers:
                cx, cy = frame.point(origin[0] + (a + b) / 2, origin[1] + y0 + depth / 2)
                px, py = prev_centers[rid]
                score += 0.6 * math.hypot(cx - px, cy - py)
        cand = _BandResult(order, cuts, score, hard_ok)
        if best is None or (cand.hard_ok, -cand.score) > (best.hard_ok, -best.score):
            best = cand
    assert best is not None
    return best


def _carve(
    r: tuple[float, float, float, float],
    parent_id: str,
    children: list[Room],
    band: str,
    at: int,
    count: int,
    has_hall: bool,
) -> dict[str, tuple[float, float, float, float]]:
    """Split a block into its parent and nested rooms (all rectangles, local frame).

    Normally the nested rooms take a full-depth strip off one end of the block, stacked with any
    bathroom on the outer (exterior) edge so it can have a window. Every nested room shares its
    long side with the parent, where building.py puts its only door. Without a hall the front
    band is a door chain, so a block in the middle of the chain keeps both ends and gives its
    nested rooms a strip along the back wall instead.
    """
    x0, y0, x1, y1 = r
    depth, width = y1 - y0, x1 - x0
    kids = sorted(children, key=lambda c: c.type != "bathroom")  # bathrooms on the outer edge
    total = sum(_gross(c) for c in kids)
    out: dict[str, tuple[float, float, float, float]] = {}
    if has_hall or at == 0 or at == count - 1:
        sw = min(slice_width(kids, depth), snap(width - 1.0))
        if at == 0 and count > 1:  # outer end of the band: the strip gets the side wall
            sx0, sx1 = x0, x0 + sw
            out[parent_id] = (round(x0 + sw, 3), y0, x1, y1)
        else:
            sx0, sx1 = x1 - sw, x1
            out[parent_id] = (x0, y0, round(x1 - sw, 3), y1)
        outer_top = band == "back"  # back band: the exterior wall is at the top
        edge = y1 if outer_top else y0
        used = 0.0
        shares = stack_depths(kids, depth)
        for i, c in enumerate(kids):
            h = depth - used if i == len(kids) - 1 else max(snap(shares[i]), _min_cut(c))
            used += h
            a, b = (edge - h, edge) if outer_top else (edge, edge + h)
            out[c.id] = (round(sx0, 3), round(a, 3), round(sx1, 3), round(b, 3))
            edge = a if outer_top else b
        return out
    sd = min(snap(max(total / max(width, 0.1), max(_min_cut(c) for c in kids))), snap(depth - 1.0))
    out[parent_id] = (x0, y0, x1, round(y1 - sd, 3))
    x = x0
    for i, c in enumerate(kids):
        w = (x1 - x) if i == len(kids) - 1 else snap(width * _gross(c) / total)
        out[c.id] = (round(x, 3), round(y1 - sd, 3), round(x + w, 3), y1)
        x += w
    return out


def compute_layout(brief: Brief, previous: BuildingModel | None = None) -> Layout:
    lot = brief.lot
    frame = Frame(brief.entrance_side, lot.width, lot.depth)
    sb = lot.setbacks
    build_w = frame.local_w - 2 * sb.sides
    build_d = frame.local_d - sb.front - sb.back
    if build_w < 4 or build_d < 4:
        raise LayoutError(
            f"setbacks leave a buildable area of {build_w:.1f} x {build_d:.1f} m; it must be at least 4 x 4 m"
        )

    explicit, implicit = _adjacency_pairs(brief)
    blocks = _blocks(brief)
    prev_centers: dict[str, tuple[float, float]] = {}
    prev_front: list[str] | None = None
    prev_back: list[str] | None = None
    if previous is not None:
        for pr in previous.rooms:
            x0, y0, x1, y1 = pr.rect
            prev_centers[pr.id] = ((x0 + x1) / 2, (y0 + y1) / 2)

    best: Layout | None = None
    best_key: tuple[bool, float] | None = None
    infeasible_reasons: list[str] = []
    for assign in _band_assignments(brief):
        top = brief.top_level()
        front_ids = [r.id for r in top if assign[r.id] == "front"]
        back_ids = [r.id for r in top if assign[r.id] == "back"]
        af = sum(blocks.gross[i] for i in front_ids)
        ab = sum(blocks.gross[i] for i in back_ids)
        hall = CORRIDOR_WIDTH if back_ids else 0.0
        area = af + ab
        zone_penalty = 3.0 * sum(1 for r in top if assign[r.id] != _zone(r) and r.side is None)

        ideal_w = math.sqrt(area * 1.5)
        widths = sorted({snap(min(build_w, max(6.0, ideal_w * f))) for f in (0.85, 1.0, 1.15, 1.3)})
        for W in widths:
            df = snap(max(af / W, max(blocks.min_depth[i] for i in front_ids)))
            db = snap(max(ab / W, max(blocks.min_depth[i] for i in back_ids))) if back_ids else 0.0
            D = round(df + hall + db, 3)
            if D > build_d:
                infeasible_reasons.append(
                    f"rooms need about {area:.0f} m² of floor plus a hall; at {W:.1f} m wide the house is "
                    f"{D:.1f} m deep but the buildable depth is {build_d:.1f} m"
                )
                continue
            origin = (snap((frame.local_w - W) / 2), sb.front)
            fr = _place_band(brief, blocks, front_ids, W, df, 0.0, explicit, implicit, prev_centers, frame, origin, prev_front)
            br = (
                _place_band(brief, blocks, back_ids, W, db, df + hall, explicit, implicit, prev_centers, frame, origin, prev_back)
                if back_ids
                else _BandResult((), [], 0.0, True)
            )
            hard_ok = fr.hard_ok and br.hard_ok
            score = fr.score + br.score + zone_penalty + 0.05 * abs(W / max(D, 0.1) - 1.5)
            key = (hard_ok, -score)
            if best_key is not None and key <= best_key:
                continue
            ox, oy = origin
            local_rects: dict[str, tuple[float, float, float, float]] = {}
            for rid, a, b in fr.cuts:
                local_rects[rid] = (ox + a, oy, ox + b, oy + df)
            if back_ids:
                local_rects["hall"] = (ox, oy + df, ox + W, oy + df + hall)
                for rid, a, b in br.cuts:
                    local_rects[rid] = (ox + a, oy + df + hall, ox + b, oy + D)
            for band, res in (("front", fr), ("back", br)):
                for at, rid in enumerate(res.order):
                    if blocks.children[rid]:
                        local_rects.update(_carve(local_rects[rid], rid, blocks.children[rid], band, at,
                                                  len(res.order), bool(back_ids)))
            best_key = key
            best = Layout(
                frame=frame,
                footprint=frame.rect((ox, oy, ox + W, oy + D)),
                rooms={k: frame.rect(v) for k, v in local_rects.items()},
                front=list(fr.order),
                back=list(br.order),
                score=round(score, 3),
                notes=[] if hard_ok else ["no candidate met every hard rule; showing the closest"],
            )
    if best is None:
        raise LayoutError(infeasible_reasons[0] if infeasible_reasons else "no layout candidate")
    return best
