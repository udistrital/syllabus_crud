package main

import (
	"encoding/base64"
	"strings"
	"testing"
)

func datosMinimos() map[string]any {
	return map[string]any{
		"nombre_espacio_academico":   "Espacio de prueba",
		"cod_espacio_academico":      "123",
		"nombre_proyecto_curricular": "Proyecto de prueba",
		"nombre_facultad":            "Facultad de prueba",
	}
}

func documentoBase64(t *testing.T, data map[string]any) string {
	t.Helper()
	respuesta, status := createTemplate(data)
	if status != 200 {
		t.Fatalf("status inesperado: %d", status)
	}
	body, ok := respuesta["body"].(map[string]any)
	if !ok {
		t.Fatalf("respuesta sin body")
	}
	documento, _ := body["Data"].(string)
	if documento == "" {
		t.Fatalf("documento vacío")
	}
	return documento
}

func TestCreateTemplateGeneraPDF(t *testing.T) {
	documento := documentoBase64(t, datosMinimos())
	raw, err := base64.StdEncoding.DecodeString(documento)
	if err != nil {
		t.Fatalf("base64 inválido: %v", err)
	}
	if !strings.HasPrefix(string(raw), "%PDF") {
		t.Fatalf("no es un PDF válido")
	}
}

func TestCreateTemplateConMarcaDeAgua(t *testing.T) {
	data := datosMinimos()
	data["marca_agua"] = "BORRADOR - NO ES LA VERSIÓN FINAL"

	conMarca, err := base64.StdEncoding.DecodeString(documentoBase64(t, data))
	if err != nil {
		t.Fatalf("base64 inválido: %v", err)
	}
	sinMarca, err := base64.StdEncoding.DecodeString(documentoBase64(t, datosMinimos()))
	if err != nil {
		t.Fatalf("base64 inválido: %v", err)
	}

	if !strings.HasPrefix(string(conMarca), "%PDF") {
		t.Fatalf("el PDF con marca no es válido")
	}
	// SetAlpha emite un objeto /ExtGState (diccionario sin comprimir); sin marca
	// no debería aparecer.
	if !strings.Contains(string(conMarca), "/ExtGState") {
		t.Fatalf("el PDF con marca no incluye transparencia (/ExtGState)")
	}
	if strings.Contains(string(sinMarca), "/ExtGState") {
		t.Fatalf("el PDF sin marca no debería incluir transparencia (/ExtGState)")
	}
}
