"""The checks every researched price must pass, and the region pinned by code. Ported from the Engine-era research cases."""

import pytest
from price_fixtures import find

from fga_agent.domain.model.price_find import PriceFind
from fga_agent.service.helper.price_checker import PriceCheckerService
from fga_agent.service.helper.region_pinner import RegionPinnerService

checker = PriceCheckerService(usd_vnd=25000)


def found(**kw) -> PriceFind:
    return PriceFind(**{"material_spec": "", "date": "", **find(**kw)})


def test_quote_must_be_on_the_page_and_carry_the_number():
    page = "Bảng giá. Gạch lát nền 60x60: 250.000đ/m2 tại TP.HCM"
    assert checker.quote_on_page(found(), page)
    assert not checker.quote_on_page(found(quote="Gạch lát nền 60x60: 999.000đ/m2"), page)  # invented quote
    assert not checker.quote_on_page(found(price=480000), page)  # quote is real but is not that number
    assert checker.quote_on_page(found(price=150000, quote="sơn 150k/m2"), "giá sơn 150k/m2 hôm nay")
    assert checker.quote_on_page(found(), "BẢNG GIÁ.   gạch lát nền 60x60:  250.000đ/m2")  # case and spacing do not matter


def test_units_and_currency_convert_or_are_rejected():
    assert checker.rate_in_vnd(found(unit="m²"), "m2") == 250000
    assert checker.rate_in_vnd(found(unit="đ/m2"), "m2") == 250000  # the currency prefix and slash are stripped
    assert checker.rate_in_vnd(found(unit="widgets"), "m2") is None
    assert checker.rate_in_vnd(found(price=18_000_000, unit="tấn"), "kg") == 18000
    assert checker.rate_in_vnd(found(price=10, currency="USD", unit="m2"), "m2") == 10 * 25000
    assert checker.rate_in_vnd(found(unit="m3"), "m2") is None
    assert checker.rate_in_vnd(found(currency="EUR"), "m2") is None
    assert checker.rate_in_vnd(found(unit="cái"), "each") == 250000


def test_aggregation_drops_outliers_and_flags_thin_evidence():
    r = checker.range_of([200, 220, 240, 260, 9000])
    assert r.confidence == "ok" and r.low <= r.base <= r.high and r.base == 230 and r.high < 300
    r = checker.range_of([100])
    assert r.confidence == "low_confidence" and r.low < r.base < r.high
    assert checker.range_of([]).confidence == "none"
    assert checker.plausible([200, 220, 240, 9000]) == [True, True, True, False]


def test_only_well_formed_finds_are_read_from_the_models_answer():
    good = find()
    items = [good, find(price=0), find(price=-5), find(quote="ab"), find(quote="x" * 401), {"price": "250000", "quote": "abc"},
             "not a find", {"price": True, "quote": "abcd"}, {"price": 10, "quote": "abc", "unit": None}]
    got = checker.finds_from(items)
    assert [f.price for f in got] == [250000, 10] and got[1].unit == "" and got[1].currency == ""
    assert checker.finds_from("nothing") == [] and checker.finds_from(None) == []
    assert len(checker.finds_from([good] * 9)) == 6


@pytest.mark.parametrize("query,region,want", [
    ("đơn giá gạch lát nền", "Hanoi", "đơn giá gạch lát nền Hanoi"),  # the region is appended by code
    ("đơn giá gạch lát nền Hanoi", "Hanoi", "đơn giá gạch lát nền Hanoi"),
    ("giá gạch ở Hà Nội", "Hanoi", "giá gạch ở Hà Nội"),  # the same city under another spelling is kept as it is
    ("giá gạch ở Đà Nẵng", "Hanoi", None),  # a query that names another city is dropped
    ("floor tile price", "", "floor tile price"),
    ("  floor   tile  ", "Da Nang", "floor tile Da Nang"),
])
def test_the_region_is_pinned_by_code(query, region, want):
    assert RegionPinnerService().pin(query, region) == want
