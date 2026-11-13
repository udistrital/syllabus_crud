#!/usr/bin/env bash
# Construye la capa Lambda con las librerias nativas (pango, cairo, gdk-pixbuf,
# fontconfig, harfbuzz, freetype) y las fuentes que necesita WeasyPrint en el
# runtime python3.12 (Amazon Linux 2023). Requiere Docker solo en tiempo de
# build; el resultado son los directorios lib/ y fonts/ junto a este script.
#
# Variables que debe recibir la funcion:
#   LD_LIBRARY_PATH=/opt/lib
#   FONTCONFIG_PATH=/opt/fonts
#   GDK_PIXBUF_MODULE_FILE=/opt/lib/gdk-pixbuf-2.0/2.10.0/loaders.cache
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IMAGE="${IMAGE:-public.ecr.aws/amazonlinux/amazonlinux:2023}"
STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT

cat > "$STAGE/inner.sh" <<'INNER_EOF'
set -euo pipefail
dnf -y install \
    pango cairo gdk-pixbuf2 fontconfig harfbuzz freetype \
    liberation-sans-fonts dejavu-sans-fonts \
    >/dev/null

mkdir -p /stage/lib /stage/fonts
: > /stage/lib/.seen

is_core() {
    case "$1" in
        */ld-*.so*|*/libc.so.*|*/libm.so.*|*/libdl.so.*|*/librt.so.*) return 0 ;;
        */libpthread.so.*|*/libresolv.so.*|*/libutil.so.*|*/libnsl.so.*|*/libanl.so.*) return 0 ;;
    esac
    return 1
}

add_lib() {
    local p="$1" b d
    [ -f "$p" ] || return 0
    b="$(basename "$p")"
    is_core "$p" && return 0
    grep -qxF "$b" /stage/lib/.seen && return 0
    echo "$b" >> /stage/lib/.seen
    cp -L "$p" "/stage/lib/$b"
    while IFS= read -r d; do add_lib "$d"; done < <(ldd "$p" 2>/dev/null | awk '/=> \//{print $3}')
}

for so in /usr/lib64/libpangocairo-1.0.so.0 /usr/lib64/libpango-1.0.so.0 \
          /usr/lib64/libpangoft2-1.0.so.0 /usr/lib64/libcairo.so.2 \
          /usr/lib64/libgdk_pixbuf-2.0.so.0 /usr/lib64/libfontconfig.so.1 \
          /usr/lib64/libharfbuzz.so.0 /usr/lib64/libharfbuzz-subset.so.0 \
          /usr/lib64/libfreetype.so.6; do
    add_lib "$so"
done

# Modulos cargadores de gdk-pixbuf (PNG/JPEG/TIFF) con cache generado y
# relocalizado a /opt (el %ghost loaders.cache del rpm no siempre se ejecuta
# cuando gdk-pixbuf2 entra como dependencia).
QL="$(command -v gdk-pixbuf-query-loaders-64 || command -v gdk-pixbuf-query-loaders || true)"
[ -n "$QL" ] || { echo "ERROR: gdk-pixbuf-query-loaders no esta disponible" >&2; exit 1; }

for vdir in /usr/lib64/gdk-pixbuf-2.0/*/; do
    vname="$(basename "$vdir")"
    mkdir -p "/stage/lib/gdk-pixbuf-2.0/$vname/loaders"
    cp -a "$vdir"loaders/. "/stage/lib/gdk-pixbuf-2.0/$vname/loaders/"
    for m in "/stage/lib/gdk-pixbuf-2.0/$vname/loaders"/*.so; do
        [ -e "$m" ] || continue
        while IFS= read -r d; do add_lib "$d"; done < <(ldd "$m" 2>/dev/null | awk '/=> \//{print $3}')
    done
    "$QL" | sed -e 's|/usr/lib64/gdk-pixbuf-2.0|/opt/lib/gdk-pixbuf-2.0|g' \
        > "/stage/lib/gdk-pixbuf-2.0/$vname/loaders.cache"
done

cp -a /usr/share/fonts/liberation-sans /usr/share/fonts/dejavu-sans-fonts /stage/fonts/

cat > /stage/fonts/fonts.conf <<'XML'
<?xml version="1.0"?>
<!DOCTYPE fontconfig SYSTEM "fonts.dtd">
<fontconfig>
  <dir>/opt/fonts</dir>
  <cachedir>/tmp/fc</cachedir>
</fontconfig>
XML

rm -f /stage/lib/.seen
INNER_EOF

docker run --rm \
    -e HOST_UID="$(id -u)" -e HOST_GID="$(id -g)" \
    -v "$STAGE":/stage \
    "$IMAGE" bash -c 'bash /stage/inner.sh; rc=$?; chown -R "$HOST_UID:$HOST_GID" /stage; exit "$rc"'

rm -rf "$SCRIPT_DIR/lib" "$SCRIPT_DIR/fonts"
mv "$STAGE/lib" "$STAGE/fonts" "$SCRIPT_DIR/"
du -sh "$SCRIPT_DIR/lib" "$SCRIPT_DIR/fonts"
