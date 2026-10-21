import base64
import json
import os
import re

import nh3
from jinja2 import Template
from weasyprint import HTML
from weasyprint.urls import URLFetcher

ASSETS_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "assets")
TEMPLATE_PATH = os.path.join(ASSETS_DIR, "plantilla_syllabus.html")

with open(TEMPLATE_PATH, encoding="utf-8") as template_file:
    TEMPLATE_STR = template_file.read()

MAX_BODY_BYTES = 5 * 1024 * 1024
MAX_FIELD_CHARS = 250_000
MAX_IMAGE_BYTES = 2 * 1024 * 1024

ALLOWED_TAGS = frozenset({
    "p", "br", "div", "span", "strong", "b", "em", "i", "u", "s",
    "ul", "ol", "li", "h1", "h2", "h3", "h4", "h5", "h6",
    "table", "thead", "tbody", "tfoot", "caption", "tr", "td", "th",
    "img", "a", "blockquote", "pre", "code", "hr", "sup", "sub",
})

ALLOWED_ATTRIBUTES = {
    "*": {"style", "class", "title"},
    "a": {"href"},
    "img": {"src", "alt", "width", "height"},
    "td": {"colspan", "rowspan"},
    "th": {"colspan", "rowspan"},
}

ALLOWED_STYLE_PROPERTIES = frozenset({
    "color", "background-color", "text-align", "text-decoration",
    "font-family", "font-size", "font-style", "font-weight",
    "width", "height", "vertical-align",
    "border", "border-collapse", "border-color", "border-spacing",
    "border-style", "border-width", "border-left", "border-right",
    "border-top", "border-bottom",
    "padding", "padding-left", "padding-right", "padding-top", "padding-bottom",
    "margin", "margin-left", "margin-right", "margin-top", "margin-bottom",
    "list-style-type", "white-space",
})

DATA_IMAGE_RE = re.compile(r"data:image/[\w.+-]+;base64,([A-Za-z0-9+/=]+)")
DATA_URI_RE = re.compile(r"data:[\w./+-]*;base64,[A-Za-z0-9+/=]+")


DATA_URL_FETCHER = URLFetcher(allowed_protocols={"data"})


def _asset_data_uri(filename: str, mime: str) -> str:
    with open(os.path.join(ASSETS_DIR, filename), "rb") as fh:
        return "data:%s;base64,%s" % (mime, base64.b64encode(fh.read()).decode("ascii"))


ASSETS = {
    "logo_left": _asset_data_uri("logo_izquierdo.png", "image/png"),
    "logo_sigud": _asset_data_uri("logo_sigud.jpg", "image/jpeg"),
}


def sanitize_html(value: str) -> str:
    return nh3.clean(
        value,
        tags=ALLOWED_TAGS,
        attributes=ALLOWED_ATTRIBUTES,
        url_schemes={"data"},
        strip_comments=True,
        filter_style_properties=ALLOWED_STYLE_PROPERTIES,
    )


def sanitize_payload(obj):
    if isinstance(obj, str):
        return sanitize_html(obj)
    if isinstance(obj, dict):
        return {
            key: (value if key == "template_str" else sanitize_payload(value))
            for key, value in obj.items()
        }
    if isinstance(obj, list):
        return [sanitize_payload(item) for item in obj]
    return obj


def find_oversized_field(obj, path=""):
    if isinstance(obj, str):
        effective_len = len(DATA_URI_RE.sub("", obj))
        return path if effective_len > MAX_FIELD_CHARS else None
    if isinstance(obj, dict):
        for key, value in obj.items():
            found = find_oversized_field(value, f"{path}.{key}" if path else str(key))
            if found:
                return found
    if isinstance(obj, list):
        for index, item in enumerate(obj):
            found = find_oversized_field(item, f"{path}[{index}]")
            if found:
                return found
    return None


def has_oversized_image(html: str) -> bool:
    for match in DATA_IMAGE_RE.finditer(html):
        if len(match.group(1)) * 3 // 4 > MAX_IMAGE_BYTES:
            return True
    return False


def build_competencia_groups(propositos):
    grupos = []
    vistos = {}
    if not isinstance(propositos, list):
        return grupos
    for prop in propositos:
        if not isinstance(prop, dict):
            continue
        nombre = str(prop.get("competencia") or "")
        if not nombre:
            continue
        if nombre not in vistos:
            vistos[nombre] = {"nombre": nombre, "items": []}
            grupos.append(vistos[nombre])
        vistos[nombre]["items"].append(prop)
    return grupos


def resolve_pfa_mode(propositos):
    """Replica la seleccion exclusiva de rama de la funcion gofpdf."""
    if not isinstance(propositos, list) or not propositos:
        return "empty"
    p0 = propositos[0]
    if not isinstance(p0, dict):
        return "v3"
    if "pfa_programa" in p0:
        return "blocks"
    if "propositos_formación_legacy" in p0:
        return "legacy"
    return "v3"


def make_pdf(html: str):
    """Generate a PDF file from a string of HTML"""
    print("Generating document")
    html_doc = HTML(string=html, url_fetcher=DATA_URL_FETCHER)
    byte_doc = html_doc.write_pdf(presentational_hints=True)
    encoded = base64.b64encode(byte_doc)
    print("Generated document")
    return encoded.decode("utf-8")


def render_html(template_str: str, data: dict):
    context = dict(data)
    context.setdefault("assets", ASSETS)
    propositos = data.get("propositos")
    context.setdefault("pfa_mode", resolve_pfa_mode(propositos))
    context.setdefault("competencia_grupos", build_competencia_groups(propositos))
    template = Template(template_str)
    html_result = template.render(**context)
    return html_result


def parse_body(event) -> tuple:
    try:
        return json.loads(event["body"]), None
    except Exception as ex:
        return None, ex


def format_response(status_code: int, message: str, success: bool, data=None):
    """Envelope anidado identico al de la funcion gofpdf (Data es b64 plano)."""
    envelope = {
        "Success": success,
        "Status": status_code,
        "Message": message,
    }
    if success:
        envelope["Data"] = data
    body = {"statusCode": status_code, "body": envelope}
    return {"statusCode": status_code, "body": json.dumps(body)}


def lambda_handler(event, context):
    try:
        raw_body = event.get("body")
        if raw_body is None:
            return format_response(
                400,
                "Error in syllabus template lambda! Detail: missing request body",
                False)
        if len(raw_body.encode("utf-8")) > MAX_BODY_BYTES:
            return format_response(
                400,
                f"Error in syllabus template lambda! Detail: request body exceeds {MAX_BODY_BYTES} bytes",
                False)
        data, error = parse_body(event)
        if error is not None:
            print(error)
            return format_response(
                400,
                "Error in syllabus template lambda! Detail: Error in input data",
                False)
        data = sanitize_payload(data)
        oversized = find_oversized_field(data)
        if oversized is not None:
            return format_response(
                400,
                f"Error in syllabus template lambda! Detail: field '{oversized}' exceeds {MAX_FIELD_CHARS} characters",
                False)
        print("Starting template rendering")
        template_str = data.get("template_str") or TEMPLATE_STR
        # Render template
        rendered_template = render_html(template_str, data)
        print("Finished rendering of the template")
        if has_oversized_image(rendered_template):
            return format_response(
                400,
                f"Error in syllabus template lambda! Detail: embedded image exceeds {MAX_IMAGE_BYTES} bytes decoded",
                False)
        result_doc_64 = make_pdf(rendered_template)
        return format_response(
            200,
            "Syllabus Template OK",
            True,
            result_doc_64)
    except Exception as ex:
        print(f"Error in syllabus template lambda. Detail: {ex}")
        return format_response(
            500,
            f"Error in syllabus template lambda! Detail: {ex}",
            False)
