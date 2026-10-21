"""Pruebas de contrato (Fase 0) de la lambda html_2_pdf: envelope tipo gofpdf,
saneado de HTML, politica data:image/* y limites de tamano.
"""

import base64
import json
import os
import sys

import pytest

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from src.handlers.html_2_pdf import app  # noqa: E402

EVENTS_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "events")

PNG_1PX = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="


def _load_event(name):
    with open(os.path.join(EVENTS_DIR, name), encoding="utf-8") as fh:
        return json.load(fh)


def _event(data):
    return {"httpMethod": "POST", "body": json.dumps(data, ensure_ascii=False)}


def _response_body(response):
    outer = json.loads(response["body"])
    assert outer["statusCode"] == response["statusCode"]
    return outer["body"]


def test_min_event_ok():
    response = app.lambda_handler(_load_event("html2pdf_min.json"), None)
    assert response["statusCode"] == 200
    body = _response_body(response)
    assert body["Success"] is True
    assert base64.b64decode(body["Data"]).startswith(b"%PDF-")


def test_rich_event_with_table_and_image_ok():
    response = app.lambda_handler(_load_event("html2pdf_event.json"), None)
    assert response["statusCode"] == 200
    body = _response_body(response)
    assert base64.b64decode(body["Data"]).startswith(b"%PDF-")


def test_competencias_event_renders_grouped_table():
    response = app.lambda_handler(_load_event("html2pdf_competencias.json"), None)
    assert response["statusCode"] == 200
    body = _response_body(response)
    assert base64.b64decode(body["Data"]).startswith(b"%PDF-")


def test_pfa_branch_mode():
    assert app.resolve_pfa_mode(None) == "empty"
    assert app.resolve_pfa_mode([]) == "empty"
    assert app.resolve_pfa_mode([{"pfa_programa": "x"}]) == "blocks"
    assert app.resolve_pfa_mode([{"propositos_formación_legacy": ["a", "b", "c"]}]) == "legacy"
    assert app.resolve_pfa_mode([{"competencia": "C", "dominio": "B"}]) == "v3"


def test_competencia_groups_preserve_order():
    rows = [
        {"competencia": "B", "id": 1}, {"competencia": "A", "id": 2},
        {"competencia": "B", "id": 3},
    ]
    grupos = app.build_competencia_groups(rows)
    assert [g["nombre"] for g in grupos] == ["B", "A"]
    assert [it["id"] for it in grupos[0]["items"]] == [1, 3]


def test_success_envelope_matches_gofpdf_shape():
    response = app.lambda_handler(_event({"nombre_espacio_academico": "X"}), None)
    body = _response_body(response)
    assert set(body.keys()) == {"Success", "Status", "Message", "Data"}
    assert isinstance(body["Data"], str)


def test_error_envelope_has_no_data():
    response = app.lambda_handler({"httpMethod": "POST"}, None)
    body = _response_body(response)
    assert response["statusCode"] == 400
    assert body["Success"] is False
    assert "Data" not in body


def test_missing_body_is_400():
    response = app.lambda_handler({"httpMethod": "POST"}, None)
    assert response["statusCode"] == 400


def test_invalid_json_body_is_400():
    response = app.lambda_handler({"body": "{no es json"}, None)
    assert response["statusCode"] == 400


def test_body_over_limit_is_400():
    huge = {"justificacion": "x" * (app.MAX_BODY_BYTES + 10)}
    response = app.lambda_handler(_event(huge), None)
    assert response["statusCode"] == 400
    assert "request body exceeds" in _response_body(response)["Message"]


def test_field_over_limit_is_400():
    response = app.lambda_handler(
        _event({"justificacion": "<p>" + "a" * (app.MAX_FIELD_CHARS + 1) + "</p>"}), None)
    assert response["statusCode"] == 400
    assert "justificacion" in _response_body(response)["Message"]


def test_image_over_limit_is_400():
    fake_b64 = "A" * ((app.MAX_IMAGE_BYTES * 4 // 3) + 1000)
    payload = {"justificacion": f'<img src="data:image/png;base64,{fake_b64}">'}
    response = app.lambda_handler(_event(payload), None)
    assert response["statusCode"] == 400
    assert "image exceeds" in _response_body(response)["Message"]


def test_template_str_override_renders_custom_markup():
    payload = {
        "template_str": "<html><body><h1>{{nombre_espacio_academico}}</h1></body></html>",
        "nombre_espacio_academico": "Topolog&iacute;a",
    }
    response = app.lambda_handler(_event(payload), None)
    assert response["statusCode"] == 200
    assert base64.b64decode(_response_body(response)["Data"]).startswith(b"%PDF-")


@pytest.mark.parametrize("payload,expected_absent", [
    ('<p>ok</p><script>alert(1)</script>', "script"),
    ("<p onclick=\"alert(1)\">hola</p>", "onclick"),
    ('<img src="https://evil.example/x.png">', "evil.example"),
    ('<a href="javascript:alert(1)">click</a>', "javascript:"),
    ('<div style="position:fixed;top:0">x</div>', "position"),
    ("<!-- comentario -->", "comentario"),
])
def test_sanitize_removes_forbidden(payload, expected_absent):
    assert expected_absent not in app.sanitize_html(payload)


def test_sanitize_keeps_allowed_markup():
    cleaned = app.sanitize_html(
        f'<table><tr><td colspan="2" style="color:#333; background-image:url(\'http://e/x\')">'
        '<strong>n</strong></td></tr></table>'
        f'<img src="data:image/png;base64,{PNG_1PX}">'
    )
    assert "<table>" in cleaned
    assert 'colspan="2"' in cleaned
    assert "color:#333" in cleaned
    assert "background-image" not in cleaned
    assert "<strong>n</strong>" in cleaned
    assert "data:image/png;base64," in cleaned


def test_plain_text_passes_sanitizer():
    assert app.sanitize_html("Presupuesto: 5 < 8 y 9 > 3") == \
        "Presupuesto: 5 &lt; 8 y 9 &gt; 3"


def test_find_oversized_field_walks_nested():
    big = "x" * (app.MAX_FIELD_CHARS + 1)
    assert app.find_oversized_field({"a": [{"b": big}]}) == "a[0].b"
    assert app.find_oversized_field({"a": [{"b": "chico"}]}) is None


def test_assets_injected_as_data_uris():
    assert app.ASSETS["logo_left"].startswith("data:image/png;base64,")
    assert app.ASSETS["logo_sigud"].startswith("data:image/jpeg;base64,")
    rendered = app.render_html(app.TEMPLATE_STR, {})
    assert app.ASSETS["logo_left"] in rendered
    assert app.ASSETS["logo_sigud"] in rendered
    assert "raw.githubusercontent.com" not in app.TEMPLATE_STR


def test_rich_pdf_embeds_images_and_no_remote_fetch():
    response = app.lambda_handler(_load_event("html2pdf_event.json"), None)
    inner = _response_body(response)
    pdf = base64.b64decode(inner["Data"])
    assert pdf.count(b"/Subtype /Image") + pdf.count(b"/Subtype/Image") >= 3


def test_watermark_and_inst_headlines_rendered():
    response = app.lambda_handler(_load_event("html2pdf_event.json"), None)
    assert response["statusCode"] == 200
    rendered = app.render_html(app.TEMPLATE_STR, {"marca_agua": "BORRADOR", "justificacion": "x"})
    assert "FORMATO DE SYLLABUS" in rendered
    assert "IDENTIFICACIÓN INSTITUCIONAL" in rendered
    assert "Borrador".upper() in rendered
