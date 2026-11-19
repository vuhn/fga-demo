"""Deterministic pricing: takeoff lines x researched rates, then the PRD cascade. The LLM never
touches these numbers. A line with no rate is `unpriced`: excluded from the total and listed in the
warnings, never priced at zero silently."""

from __future__ import annotations

from .estimate_models import Estimate, Markup, PricedLine, RateResult, SessionState, Takeoff

# mid-range of the PRD; used only when the user agrees ("use typical")
DEFAULT_MARKUPS = {
    "overhead_percent": 6.5,   # site overhead and temporary works, 5-8%
    "profit_percent": 7.5,     # contractor profit, 5-10%
    "design_percent": 5.5,     # design, supervision, permits, 3-8%
    "contingency_percent": 7.5,  # 5-10%
    "vat_percent": 8.0,        # the user confirms the rate in force
}
CONTINGENCY_EXTRA_MAX = 5.0  # added at 100% assumed or low-confidence cost


def _pct(state: SessionState, name: str) -> float:
    f = state.facts.get(name)
    try:
        return float(f.value) if f is not None else DEFAULT_MARKUPS[name]
    except (TypeError, ValueError):
        return DEFAULT_MARKUPS[name]


def price(tk: Takeoff, rates: dict[str, RateResult], state: SessionState) -> Estimate:
    est = Estimate()
    est.rates = [r for r in rates.values()]
    direct = {"low": 0.0, "base": 0.0, "high": 0.0}
    shaky = 0.0  # base cost of assumed or low-confidence lines
    pkgs: dict[str, dict[str, float]] = {}

    for ln in tk.lines:
        r = rates.get(ln.code)
        pl = PricedLine(code=ln.code, package=ln.package, description=ln.description, quantity=ln.quantity,
                        unit=ln.unit, basis=ln.basis, confidence=ln.confidence)
        if r is None or r.base is None:
            pl.priced = False
            est.warnings.append(f"No price found for {ln.description} ({ln.quantity:g} {ln.unit}); it is not in the total. Enter a rate to include it.")
        else:
            lo = r.low if r.low is not None else r.base
            hi = r.high if r.high is not None else r.base
            pl.rate_low, pl.rate_base, pl.rate_high = lo, r.base, hi
            pl.low, pl.base, pl.high = ln.quantity * lo, ln.quantity * r.base, ln.quantity * hi
            pl.rate_confidence = r.confidence
            for k in direct:
                direct[k] += getattr(pl, k)
            p = pkgs.setdefault(ln.package, {"low": 0.0, "base": 0.0, "high": 0.0})
            for k in p:
                p[k] += getattr(pl, k)
            if ln.confidence == "assumed" or r.confidence == "low_confidence":
                shaky += pl.base
        est.lines.append(pl)

    est.packages = {k: {m: round(v) for m, v in p.items()} for k, p in pkgs.items()}
    est.direct = {k: round(v) for k, v in direct.items()}
    est.assumed_share = round(shaky / direct["base"], 3) if direct["base"] else 0.0

    contingency = _pct(state, "contingency_percent") + CONTINGENCY_EXTRA_MAX * est.assumed_share
    run = dict(direct)
    steps = [
        ("Site overhead and temporary works", _pct(state, "overhead_percent"), "direct cost", "direct"),
        ("Contractor profit", _pct(state, "profit_percent"), "direct cost + overhead", "run"),
        ("Design, supervision and permits", _pct(state, "design_percent"), "direct cost + overhead + profit", "run"),
        ("Contingency", contingency, "all of the above", "run"),
    ]
    for name, pct, label, basis in steps:
        src = direct if basis == "direct" else run
        amounts = {k: src[k] * pct / 100 for k in run}
        for k in run:
            run[k] += amounts[k]
        est.markups.append(Markup(name=name, percent=round(pct, 2), base_label=label, **{k: round(v) for k, v in amounts.items()}))
    vat = _pct(state, "vat_percent")
    vat_amounts = {k: run[k] * vat / 100 for k in run}
    est.markups.append(Markup(name="VAT", percent=round(vat, 2), base_label="all of the above", **{k: round(v) for k, v in vat_amounts.items()}))
    est.total = {k: round(run[k] + vat_amounts[k]) for k in run}

    for name, f in state.facts.items():
        if f.source == "assumed":
            est.assumptions.append(f"{name} = {f.value} (assumed, you asked me to decide)")
    if est.assumed_share:
        est.assumptions.append(f"{est.assumed_share:.0%} of the direct cost rests on assumed quantities or low-confidence rates; contingency is raised to {contingency:.1f}%.")
    est.warnings = list(tk.warnings) + est.warnings
    if any("://demo.invalid" in src.url for r in rates.values() for src in r.sources):
        est.warnings.insert(0, "DEMO DATA: the unit prices come from the fake demo search (ESTIMATE_SEARCH_PROVIDER=demo), not from the market. Do not use this total.")
    return est
