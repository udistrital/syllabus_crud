package main

import (
	"bufio"
	"bytes"

	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/phpdave11/gofpdf"
)

type PageStyle struct {
	ML                    float64 // margen izq
	MT                    float64 // margen sup
	MR                    float64 // margen der
	MB                    float64 // margen inf
	WW                    float64 // ancho area trabajo
	HW                    float64 // alto area trabajo
	HH                    float64 // alto header
	HB                    float64 // alto body
	HF                    float64 // alto footer
	WC                    float64 // ancho columna
	HR                    float64 // alto celda
	BaseColorRGB          [3]int
	SecondaryColorRGB     [3]int
	ComplementaryColorRGB [3]int
}

func getPageStyle(pdf *gofpdf.Fpdf) PageStyle {
	widthPage, heightPage := pdf.GetPageSize()
	l, t, r, b := pdf.GetMargins()
	pageStyle := PageStyle{
		ML: l,
		MT: t,
		MR: r,
		MB: b,
		WW: widthPage - l - r,
		HW: heightPage - (2 * t),
		HH: 0,
		HB: heightPage,
		HF: 0,
		WC: 0,
		HR: 6}

	// gray for headers
	pageStyle.BaseColorRGB[0] = 201
	pageStyle.BaseColorRGB[1] = 201
	pageStyle.BaseColorRGB[2] = 201

	return pageStyle
}

func FontStyle(pdf *gofpdf.Fpdf, style string, size float64, bw int, fontFamily string) {
	pdf.SetTextColor(bw, bw, bw)

	if fontFamily == "" {
		fontFamily = "Arial"
	}
	pdf.SetFont(fontFamily, style, size)
}

const (
	headerH     = 40.0
	logoIzqH    = 35.0
	logoSigudW  = 38.0
	headerLineH = 4.5
)

func aspectRatio(data []byte) float64 {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width == 0 {
		return 1
	}
	return float64(cfg.Height) / float64(cfg.Width)
}

func registerImage(pdf *gofpdf.Fpdf, name string, data []byte, imageType string) {
	reader := bytes.NewReader(data)
	if pdf.RegisterImageOptionsReader(name,
		gofpdf.ImageOptions{ImageType: imageType, AllowNegativePosition: true},
		reader) == nil {
		pdf.SetErrorf("no se pudo registrar la imagen %s", name)
	}
}

func drawImage(pdf *gofpdf.Fpdf, name string, x, y, w, h float64) {
	pdf.ImageOptions(name, x, y, w, h, false, gofpdf.ImageOptions{ImageType: ""}, 0, "")
}

func drawHeaderImages(pdf *gofpdf.Fpdf, pageStyle PageStyle) {
	registerImage(pdf, "logoIzquierdo", logoIzquierdo, "png")
	registerImage(pdf, "logoSigud", logoSigud, "jpg")

	sigudW := logoSigudW
	sigudH := logoSigudW * aspectRatio(logoSigud)
	if sigudH > headerH {
		sigudW = sigudW * (headerH / sigudH)
		sigudH = headerH
	}
	logoIzqW := logoIzqH / aspectRatio(logoIzquierdo)

	// Banda A (2WC): logo izquierdo centrado horizontal y verticalmente
	drawImage(pdf, "logoIzquierdo",
		pageStyle.ML+(2*pageStyle.WC-logoIzqW)/2,
		pageStyle.MT+(headerH-logoIzqH)/2,
		logoIzqW, logoIzqH)

	// Banda D (3WC): logo derecho centrado horizontal y verticalmente
	drawImage(pdf, "logoSigud",
		pageStyle.ML+7*pageStyle.WC+(3*pageStyle.WC-sigudW)/2,
		pageStyle.MT+(headerH-sigudH)/2,
		sigudW, sigudH)
}

func drawHeaderCellText(pdf *gofpdf.Fpdf, x, y, w, h float64, text string) {
	if text == "" {
		return
	}
	// La fuente debe estar seteada antes de llamar (SplitLines usa currentFont)
	lines := pdf.SplitLines([]byte(text), w)
	blockH := float64(len(lines)) * headerLineH
	pdf.SetXY(x, y+(h-blockH)/2) // centrado vertical del bloque
	pdf.MultiCell(w, headerLineH, text, "", "C", false)
}

func drawFixedCell(pdf *gofpdf.Fpdf, x, y, w, h float64, text, align string, maxLines int) {
	// Borde de la celda (sin relleno, como el estilo actual)
	pdf.Rect(x, y, w, h, "D")

	// La fuente debe estar seteada antes de llamar (SplitLines usa currentFont)
	lines := pdf.SplitLines([]byte(text), w-2)
	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[:maxLines]
		parts := make([]string, len(lines))
		for i, l := range lines {
			parts[i] = string(l)
		}
		text = strings.Join(parts, "\n") // truncado a maxLines
	}

	blockH := float64(len(lines)) * headerLineH
	pdf.SetXY(x+1, y+(h-blockH)/2) // centrado vertical del bloque
	pdf.MultiCell(w-2, headerLineH, text, "", align, false)
}

func drawHeaderGrid(pdf *gofpdf.Fpdf, pageStyle PageStyle, leftTexts, rightTexts [3]string) {
	tr := pdf.UnicodeTranslatorFromDescriptor("")

	x0, y0 := pageStyle.ML, pageStyle.MT

	colA := 2 * pageStyle.WC
	colB := 3 * pageStyle.WC
	colC := 2 * pageStyle.WC

	xAB := x0 + colA  // ML + 2WC
	xBC := xAB + colB // ML + 5WC
	xCD := xBC + colC // ML + 7WC

	// La altura del header se divide en 4 partes: filas 1:1:2 (B3/C3 mas altas)
	rowUnit := headerH / 4.0
	rowHeights := [3]float64{rowUnit, rowUnit, 2 * rowUnit}

	// Marco exterior obligatorio
	pdf.Rect(x0, y0, pageStyle.WW, headerH, "D")
	// Separadores verticales entre bandas
	pdf.Line(xAB, y0, xAB, y0+headerH)
	pdf.Line(xBC, y0, xBC, y0+headerH)
	pdf.Line(xCD, y0, xCD, y0+headerH)
	// Separadores horizontales solo en las bandas B y C
	acc := y0
	for i := 0; i < 2; i++ {
		acc += rowHeights[i]
		pdf.Line(xAB, acc, xCD, acc)
	}

	// Textos B1..B3 y C1..C3 (multilínea, centrados horizontal y verticalmente)
	y := y0
	for i := 0; i < 3; i++ {
		style := ""
		if i == 0 {
			style = "B" // B1: título en negrita
		}

		FontStyle(pdf, style, 9, 0, "Helvetica")
		drawHeaderCellText(pdf, xAB, y, colB, rowHeights[i], tr(leftTexts[i]))

		FontStyle(pdf, "", 9, 0, "Helvetica")
		drawHeaderCellText(pdf, xBC, y, colC, rowHeights[i], tr(rightTexts[i]))

		y += rowHeights[i]
	}
}

func headerTemplate(pdf *gofpdf.Fpdf, pageStyle PageStyle) {
	// Logos primero para que el marco/rejilla quede nitido encima
	drawHeaderImages(pdf, pageStyle)

	leftTexts := [3]string{"FORMATO DE SYLLABUS", "Macroproceso: Direccionamiento Estratégico", "Proceso: Currículo y Calidad"} // B1..B3
	rightTexts := [3]string{"Código: CC-FR-003", "Versión: 02", "Fecha de Aprobación: XX-XX-2026"}                              // C1..C3

	drawHeaderGrid(pdf, pageStyle, leftTexts, rightTexts)

	// El header ya no usa CellFormat con ln=1; reposicionar el cursor
	pdf.SetXY(pageStyle.ML, pageStyle.MT+headerH)
}

func identificationSection(pdf *gofpdf.Fpdf, pageStyle PageStyle, data map[string]any) {
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	pdf.SetX(pageStyle.ML)
	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.SetFillColor(
		pageStyle.BaseColorRGB[0],
		pageStyle.BaseColorRGB[1],
		pageStyle.BaseColorRGB[2])
	pdf.CellFormat(pageStyle.WC*10, 6, tr("IDENTIFICACIÓN DEL ESPACIO ACADÉMICO"),
		"LRT", 1, "CM", true, 0, "")

	spaceName, spaceNameOk := data["nombre_espacio_academico"]
	if !spaceNameOk || spaceName == nil {
		spaceName = ""
	}
	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.SetFillColor(
		pageStyle.BaseColorRGB[0],
		pageStyle.BaseColorRGB[1],
		pageStyle.BaseColorRGB[2])
	language, okLanguage := data["idiomas"]
	if !(okLanguage && language != nil) {
		language = ""
	}

	// Fila unica de 4 celdas: | 3WC | 3WC | 2WC | 2WC |
	rowY := pdf.GetY()
	cellH := 2 * headerLineH // 2 lineas garantizadas

	// 1) 3WC: etiqueta (negrita, centrada)
	FontStyle(pdf, "B", 9, 0, "Helvetica")
	drawFixedCell(pdf, pageStyle.ML, rowY, pageStyle.WC*3, cellH,
		tr("Nombre del espacio académico"), "LM", 2)

	// 2) 3WC: valor del nombre (centrado, truncado a 2 lineas)
	FontStyle(pdf, "", 9, 0, "Helvetica")
	drawFixedCell(pdf, pageStyle.ML+pageStyle.WC*3, rowY, pageStyle.WC*3, cellH,
		tr(fmt.Sprintf(" %v", spaceName)), "C", 2)

	// 3) 2WC: etiqueta "Idioma" (negrita, izquierda)
	FontStyle(pdf, "B", 9, 0, "Helvetica")
	drawFixedCell(pdf, pageStyle.ML+pageStyle.WC*6, rowY, pageStyle.WC*2, cellH,
		tr("Idioma"), "L", 2)

	// 4) 2WC: valor del idioma (centrado)
	FontStyle(pdf, "", 9, 0, "Helvetica")
	drawFixedCell(pdf, pageStyle.ML+pageStyle.WC*8, rowY, pageStyle.WC*2, cellH,
		tr(fmt.Sprintf(" %v", language)), "C", 2)

	// Continuar debajo de la fila
	pdf.SetXY(pageStyle.ML, rowY+cellH)

	// Código del espacio académico
	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*3, 6, tr("Código del espacio académico"),
		"LBR", 0, "LM", false, 0, "")
	spaceCod, spaceCodOk := data["cod_espacio_academico"]
	if !spaceCodOk || spaceCod == nil {
		spaceCod = ""
	}
	FontStyle(pdf, "", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC, 6, tr(fmt.Sprintf("%v", spaceCod)),
		"BR", 0, "CM", false, 0, "")

	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*3, 6, tr(" Número de créditos académicos"),
		"BR", 0, "LM", false, 0, "")
	numCredits, numCreditsOk := data["num_creditos"]
	if !numCreditsOk || numCredits == nil {
		numCredits = ""
	}
	FontStyle(pdf, "", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC, 6, tr(fmt.Sprintf("%v", numCredits)),
		"BR", 0, "CM", false, 0, "")

	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC, 6, tr(" Modalidad"),
		"BR", 0, "LM", false, 0, "")
	FontStyle(pdf, "", 9, 0, "Helvetica")
	modalidad, modalidadOk := data["modalidad"]
	if !modalidadOk || modalidad == nil {
		modalidad = ""
	}
	pdf.CellFormat(pageStyle.WC, 6, tr(fmt.Sprintf("%v", modalidad)),
		"LBR", 1, "LM", false, 0, "")

	//	Distribución horas de trabajo
	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*4, 6, tr("Distribución horas de trabajo"),
		"LBR", 0, "LM", false, 0, "")
	pdf.CellFormat(pageStyle.WC, 6, "HTD", "BR", 0, "CM", false, 0, "")
	htd, htdOk := data["htd"]
	if !htdOk || htd == nil {
		htd = ""
	}
	FontStyle(pdf, "", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC, 6, tr(fmt.Sprintf("%v", htd)),
		"BR", 0, "CM", false, 0, "")
	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC, 6, "HTC", "BR", 0, "CM", false, 0, "")
	htc, htcOk := data["htc"]
	if !htcOk || htc == nil {
		htc = ""
	}
	FontStyle(pdf, "", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC, 6, tr(fmt.Sprintf("%v", htc)),
		"BR", 0, "CM", false, 0, "")
	FontStyle(pdf, "", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC, 6, "HTA", "BR", 0, "CM", false, 0, "")
	hta, htaOk := data["hta"]
	if !htaOk || hta == nil {
		hta = ""
	}
	FontStyle(pdf, "", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC, 6, tr(fmt.Sprintf("%v", hta)),
		"BR", 1, "CM", false, 0, "")

	//	clasificacion de espacio académico, ANTES NATURALEZA
	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*3, 6, tr("Clasificación del espacio académico"),
		"LBR", 0, "LM", false, 0, "")

	// data["es_obligatorio_basico"], electivo, ..... CORREGIR MID
	naturaleza, naturalezaOk := data["naturaleza"]
	if !naturalezaOk || naturaleza == nil {
		naturaleza = ""
	}
	FontStyle(pdf, "", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*2, 6, tr(fmt.Sprintf("%v", naturaleza)),
		"BR", 0, "CM", false, 0, "")

	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*3, 6, tr(" Carácter del espacio académico"), "BR", 0, "LM", false, 0, "")
	// isTheoretical := data["es_teorico"], practico, teorico-practico, CORREGIR MID
	caracter, caracterOk := data["caracter"]
	if !caracterOk || caracter == nil {
		caracter = ""
	}
	FontStyle(pdf, "", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*2, 6, tr(fmt.Sprintf("%v", caracter)),
		"BR", 1, "CM", false, 0, "")

	pdf.CellFormat(pageStyle.WC*10, 3, "", "LBR", 1, "CM", false, 0, "")

	// pdf.CellFormat(pageStyle.WC, 6, "", "BR", 1, "CM", false, 0, "")
	// pdf.CellFormat(pageStyle.WC, 3, tr(""),
	// 	"LBR", 1, "CM", false, 0, "")

	// natureAcademicSpace(pdf, pageStyle, data)
	// characterAcademicSpace(pdf, pageStyle, data)
	// modalityAcademicSpace(pdf, pageStyle, data)
}

func suggestionsSection(pdf *gofpdf.Fpdf, pageStyle PageStyle, data map[string]any) {
	// SUGERENCIAS DE SABERES Y CONOCIMIENTOS PREVIOS
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	pdf.SetX(pageStyle.ML)
	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*10, 6, tr("Saberes y conocimientos previos"),
		"LRB", 1, "CM", true, 0, "")

	FontStyle(pdf, "", 9, 0, "Helvetica")
	suggestions, okSuggestions := data["sugerencias"]
	if okSuggestions && suggestions != nil {
		pdf.MultiCell(pageStyle.WC*10, 4.5, tr(fmt.Sprintf("%v", suggestions)),
			"LBR", "J", false)
	} else {
		pdf.CellFormat(pageStyle.WC*10, 6, tr(""), "LBR", 1, "LM", false, 0, "")
	}
}

func justificationSection(pdf *gofpdf.Fpdf, pageStyle PageStyle, data map[string]any) {
	// JUSTIFICACIÓN DEL ESPACIO ACADÉMICO
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*10, 6, tr("Justificación del espacio académico"),
		"LRB", 1, "CM", true, 0, "")

	FontStyle(pdf, "", 9, 0, "Helvetica")
	justification, okJustification := data["justificacion"]
	if okJustification && justification != nil {
		pdf.MultiCell(pageStyle.WC*10, 4.5, tr(fmt.Sprintf("%v", justification)),
			"LBR", "J", false)
	} else {
		pdf.CellFormat(pageStyle.WC*10, 6, tr(""), "LBR", 1, "LM", false, 0, "")
	}
}

func objectivesSection(pdf *gofpdf.Fpdf, pageStyle PageStyle, data map[string]any) {
	// OBJETIVOS
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*10, 6,
		tr("Objetivos del espacio académico"),
		"LBR", 1, "CM", true, 0, "")

	pdf.SetFillColor(255, 255, 255)
	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*10, 6, tr("Objetivo General"), "LR", 1, "LM", true, 0, "")

	FontStyle(pdf, "", 9, 0, "Helvetica")
	genObjective, okGenObjective := data["objetivo_general"]
	if okGenObjective && genObjective != nil {
		pdf.MultiCell(pageStyle.WC*10, 4.5, tr(fmt.Sprintf("%v \n\n", genObjective)),
			"LR", "J", false)
	} else {
		pdf.CellFormat(pageStyle.WC*10, 6, tr(""), "LR", 1, "LM", false, 0, "")
	}

	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*10, 6, tr("Objetivos Específicos"), "LR", 1, "LM", true, 0, "")

	FontStyle(pdf, "", 9, 0, "Helvetica")
	specObjective, okSpecObjective := data["objetivos_especificos"]
	if okSpecObjective && specObjective != nil {
		specObjectives := ""
		for _, objective := range specObjective.([]any) {
			specObjectives += fmt.Sprintf("•%v \n", objective)
		}
		pdf.MultiCell(pageStyle.WC*10, 4.5, tr(fmt.Sprintf("%v\n", specObjectives)),
			"LBR", "J", false)
	} else {
		pdf.CellFormat(pageStyle.WC*10, 6, "", "LBR", 1, "LM", false, 0, "")
	}
	pdf.SetFillColor(
		pageStyle.BaseColorRGB[0],
		pageStyle.BaseColorRGB[1],
		pageStyle.BaseColorRGB[2])
}

func maxFloat(vals ...float64) float64 {
	max := 0.0
	for _, v := range vals {
		if v > max {
			max = v
		}
	}
	return max
}

// Dibuja los encabezados de la tabla.
func drawHeaders(pdf *gofpdf.Fpdf, tr func(string) string, colCompetencias, colDominio, colRA, colResultados float64) {
	FontStyle(pdf, "B", 8, 0, "Helvetica") // Usando tu helper FontStyle
	pdf.SetFillColor(201, 201, 201)        // Color gris como en tu getPageStyle
	pdf.CellFormat(colCompetencias, 8, tr("Competencias"), "LTRB", 0, "CM", true, 0, "")
	pdf.CellFormat(colDominio, 8, tr("Dominio-Nivel"), "TRB", 0, "CM", true, 0, "")
	pdf.CellFormat(colRA, 8, tr("RA"), "TRB", 0, "CM", true, 0, "")
	pdf.CellFormat(colResultados, 8, tr("Resultados de Aprendizaje"), "TRB", 1, "CM", true, 0, "")
}

func purposeSection(pdf *gofpdf.Fpdf, pageStyle PageStyle, data map[string]any) {
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	pdf.SetX(pageStyle.ML)
	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.SetFillColor(pageStyle.BaseColorRGB[0], pageStyle.BaseColorRGB[1], pageStyle.BaseColorRGB[2])
	pdf.CellFormat(pageStyle.WW, 6, // Usando pageStyle.WW para el ancho total
		tr("Propósitos de Formación y Aprendizaje (PFA)"),
		"LRB", 1, "CM", true, 0, "")

	propositos, propositosOk := data["propositos"]
	if !propositosOk || propositos == nil {
		FontStyle(pdf, "", 9, 0, "Helvetica")
		pdf.CellFormat(pageStyle.WW, 6, tr("No hay propósitos de formación definidos"),
			"LBR", 1, "CM", false, 0, "")
		return
	}

	prop, _ := propositos.([]interface{})
	p, _ := prop[0].(map[string]interface{})
	// Verificar si están los propósitos en versión legacy
	_, exist := p["propositos_formación_legacy"]
	if exist {
		p_data := p["propositos_formación_legacy"].([]interface{})
		for i := 0; i < len(p_data); i += 3 {
			FontStyle(pdf, "B", 9, 0, "Helvetica")
			pdf.CellFormat(pageStyle.WC*10, 6, tr(fmt.Sprintf("  PROPÓSITO %v\n", (i+3)/3)), "LR", 1, "LM", false, 0, "")

			pdf.CellFormat(pageStyle.WC*10, 6, tr("PFA del Programa/Proyecto"), "LR", 1, "LM", false, 0, "")
			// pdf.MultiCell(pageStyle.WC*10, 4.5, tr("PFA del Programa\n"), "LBR", "J", false)
			FontStyle(pdf, "", 9, 0, "Helvetica")
			pdf.MultiCell(pageStyle.WC*10, 4.5, tr(fmt.Sprintf("%v\n", p_data[i].(string))), "LR", "J", false)

			FontStyle(pdf, "B", 9, 0, "Helvetica")
			pdf.CellFormat(pageStyle.WC*10, 6, tr("PFA de la Asignatura"), "LR", 1, "LM", false, 0, "")
			// pdf.MultiCell(pageStyle.WC*10, 4.5, tr("PFA de la Asignatura\n"), "LBR", "J", false)
			FontStyle(pdf, "", 9, 0, "Helvetica")
			pdf.MultiCell(pageStyle.WC*10, 4.5, tr(fmt.Sprintf("%v\n", p_data[i+1].(string))), "LR", "J", false)

			FontStyle(pdf, "B", 9, 0, "Helvetica")
			pdf.CellFormat(pageStyle.WC*10, 6, tr("Competencias"), "LR", 1, "LM", false, 0, "")
			// pdf.MultiCell(pageStyle.WC*10, 4.5, tr("Competencias\n"), "LBR", "J", false)
			FontStyle(pdf, "", 9, 0, "Helvetica")
			pdf.MultiCell(pageStyle.WC*10, 4.5, tr(fmt.Sprintf("%v\n \n", p_data[i+2].(string))), "LBR", "J", false)
		}

	} else {
		// sino se asume que llegan propositos de aprendizaje V3, genericos o reales
		// Anchos de columna basados en tu PageStyle.WW
		colCompetencias := pageStyle.WW * 0.3 // 30%
		colDominio := pageStyle.WW * 0.2      // 20%
		colRA := pageStyle.WW * 0.1           // 10%
		colResultados := pageStyle.WW * 0.4   // 40%

		drawHeaders(pdf, tr, colCompetencias, colDominio, colRA, colResultados)

		propositosArray := propositos.([]any)
		type CompetenciaGroup struct {
			nombre string
			items  []map[string]any
		}
		var competenciasOrdenadas []CompetenciaGroup
		competenciasVistas := make(map[string]bool)
		for _, prop := range propositosArray {
			propMap := prop.(map[string]any)
			programa := fmt.Sprintf("%v", propMap["competencia"])
			if !competenciasVistas[programa] {
				competenciasVistas[programa] = true
				grupo := CompetenciaGroup{nombre: programa, items: make([]map[string]any, 0)}
				competenciasOrdenadas = append(competenciasOrdenadas, grupo)
			}
			for i := range competenciasOrdenadas {
				if competenciasOrdenadas[i].nombre == programa {
					competenciasOrdenadas[i].items = append(competenciasOrdenadas[i].items, propMap)
					break
				}
			}
		}

		// Altura de línea base para cálculos.
		const lineHeight = 4.5
		_, pageH := pdf.GetPageSize()

		for _, grupo := range competenciasOrdenadas {
			competenciaText := grupo.nombre

			// 1. Pre-calcular la altura de cada fila para este grupo
			var rowsHeights []float64
			var totalHeightForGroup float64
			for _, resultado := range grupo.items {
				dominioStr := fmt.Sprintf("%v", resultado["dominio"])
				detalleStr := fmt.Sprintf("%v", resultado["resultado_detallado"])
				FontStyle(pdf, "", 7, 0, "Helvetica")
				linesDominio := pdf.SplitLines([]byte(tr(dominioStr)), colDominio-2) // -2 para un pequeño margen interno
				linesResultados := pdf.SplitLines([]byte(tr(detalleStr)), colResultados-2)

				// La altura de la fila es la máxima de las celdas de esa fila
				// La celda de Competencia no se incluye aquí porque su altura total es la suma de todas las filas.
				rowHeight := maxFloat(float64(len(linesDominio))*lineHeight, float64(len(linesResultados))*lineHeight, 10)
				rowsHeights = append(rowsHeights, rowHeight)
				totalHeightForGroup += rowHeight
			}

			// 2. Verificar si el bloque completo cabe en la página
			if pdf.GetY()+totalHeightForGroup > (pageH - pageStyle.MB) {
				pdf.AddPage()
				drawHeaders(pdf, tr, colCompetencias, colDominio, colRA, colResultados)
			}

			// 3. Dibujar el bloque
			startY := pdf.GetY()
			startX := pdf.GetX()

			// Dibujar la celda de competencia "fusionada"
			FontStyle(pdf, "B", 7, 0, "Helvetica")
			pdf.Rect(startX, startY, colCompetencias, totalHeightForGroup, "D")

			// Calcular posición para centrar el texto verticalmente en la celda grande
			linesCompetencia := pdf.SplitLines([]byte(tr(competenciaText)), colCompetencias-2)
			textHeight := float64(len(linesCompetencia)) * lineHeight
			yText := startY + (totalHeightForGroup-textHeight)/2
			pdf.SetXY(startX+1, yText) // +1 para margen
			pdf.MultiCell(colCompetencias-2, lineHeight, tr(competenciaText), "", "LM", false)

			// Restaurar posición para dibujar las celdas de la derecha
			pdf.SetXY(startX+colCompetencias, startY)

			for i, resultado := range grupo.items {
				rowHeight := rowsHeights[i]
				currentX, currentY := pdf.GetX(), pdf.GetY()

				dominioStr := fmt.Sprintf("%v", resultado["dominio"])
				raStr := fmt.Sprintf("%v", resultado["id"])
				detalleStr := fmt.Sprintf("%v", resultado["resultado_detallado"])

				FontStyle(pdf, "", 7, 0, "Helvetica")

				// Dibujar borde y contenido para cada celda
				pdf.Rect(currentX, currentY, colDominio, rowHeight, "D")
				pdf.SetXY(currentX+1, currentY+1)
				pdf.MultiCell(colDominio-2, lineHeight, tr(dominioStr), "", "CM", false)

				pdf.SetXY(currentX+colDominio, currentY)
				pdf.Rect(currentX+colDominio, currentY, colRA, rowHeight, "D")
				pdf.SetXY(currentX+colDominio+1, currentY+1)
				pdf.MultiCell(colRA-2, lineHeight, tr(raStr), "", "CM", false)

				pdf.SetXY(currentX+colDominio+colRA, currentY)
				pdf.Rect(currentX+colDominio+colRA, currentY, colResultados, rowHeight, "D")
				pdf.SetXY(currentX+colDominio+colRA+1, currentY+1)
				pdf.MultiCell(colResultados-2, lineHeight, tr(detalleStr), "", "LM", false)

				pdf.SetXY(startX+colCompetencias, currentY+rowHeight)
			}

			pdf.SetY(startY + totalHeightForGroup)
		}
	}
}

func thematicContentSection(pdf *gofpdf.Fpdf, pageStyle PageStyle, data map[string]any) {
	// CONTENIDOS TEMÁTICOS
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.SetX(pageStyle.ML)
	pdf.CellFormat(pageStyle.WC*10, 6, tr("Contenidos temáticos"),
		"LBR", 1, "CM", true, 0, "")
	pdf.SetFillColor(255, 255, 255)
	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*10, 6, tr("Descripción:"), "LR", 1, "LM", true, 0, "")
	FontStyle(pdf, "", 9, 0, "Helvetica")
	thematicDesc, okThematicDesc := data["contenido_tematico_descripcion"]
	if okThematicDesc && thematicDesc != nil {
		pdf.MultiCell(pageStyle.WC*10, 4.5, tr(fmt.Sprintf("%v \n\n", thematicDesc)),
			"LR", "J", false)
	} else {
		pdf.CellFormat(pageStyle.WC*10, 6, "", "LR", 1, "LM", false, 0, "")
	}

	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*10, 6, tr("Temas y subtemas:"), "LR", 1, "LM", true, 0, "")
	FontStyle(pdf, "", 9, 0, "Helvetica")
	thematicDetails, okThematicDet := data["contenido_tematico_detalle"]
	if okThematicDet && thematicDetails != nil {
		thematicDetStr := ""
		for _, topic := range thematicDetails.([]any) {
			topicAux := topic.(map[string]any)
			topicName, topicNameOk := topicAux["nombre"]
			if !topicNameOk || topicName == nil {
				thematicDetStr += fmt.Sprintf("  • \n")
			} else {
				thematicDetStr += fmt.Sprintf("  • %v \n", topicName)
			}

			subtopics, subtopicsOk := topicAux["subtemas"]
			if subtopicsOk && subtopics != nil {
				for _, subtopic := range subtopics.([]any) {
					thematicDetStr += fmt.Sprintf("      - %v \n", subtopic)
				}
			}
		}
		pdf.MultiCell(pageStyle.WC*10, 4.5, tr(fmt.Sprintf("%v\n", thematicDetStr)),
			"LBR", "J", false)
	} else {
		pdf.CellFormat(pageStyle.WC*10, 6, "", "LR", 1, "LM", false, 0, "")
	}

	pdf.SetFillColor(
		pageStyle.BaseColorRGB[0],
		pageStyle.BaseColorRGB[1],
		pageStyle.BaseColorRGB[2])
}

func strategiesSection(pdf *gofpdf.Fpdf, pageStyle PageStyle, data map[string]any) {
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*10, 6,
		tr("VII. ESTRATEGIAS DE ENSEÑANZA QUE FAVORECEN EL APRENDIZAJE"),
		"LRB", 1, "CM", true, 0, "")
	strategies, okStrategies := data["estrategias_ensenanza"]

	// Verificar si la estructura es un Array, si lo es son estrategias legacy
	estr_leg, estr_leg_ok := strategies.([]interface{})
	if estr_leg_ok {
		var parrafo_estrategias string

		pdf.SetFillColor(255, 255, 255)
		FontStyle(pdf, "", 9, 0, "Helvetica")

		// itera sobre cada elemento del array-slice y agrega al párrago final
		for _, item := range estr_leg {
			parrafo_estrategias += fmt.Sprintf(" • %v \n", item)
		}
		pdf.MultiCell(pageStyle.WC*10, 4.5, tr(fmt.Sprintf("%v\n", parrafo_estrategias)), "LBR", "J", false)

	} else if okStrategies && strategies != nil {
		strategiesMap := strategies.(map[string]any)
		FontStyle(pdf, "", 8, 0, "Helvetica")

		// Ancho de cada columna (3 columnas)
		colWidth := pageStyle.WC * 10 / 3

		// Organizar estrategias en 3x3
		strategyGrid := [][]struct {
			key   string
			label string
		}{
			// PRIMERA FILA
			{
				{"tradicional", "Tradicional"},
				{"basado_proyectos", "Basado en Proyectos"},
				{"basado_tecnologia", "Basado en Tecnología"},
			},
			// SEGUNDA FILA
			{
				{"basado_problemas", "Basado en Problemas"},
				{"colaborativo", "Colaborativo"},
				{"basado_experiencias", "Basado en Experiencias"},
			},
			// TERCERA FILA
			{
				{"aprendizaje_activo", "Aprendizaje Activo"},
				{"autodirigido", "Autodirigido"},
				{"centrado_estudiante", "Centrado en el estudiante"},
			},
		}

		for _, row := range strategyGrid {
			for _, strategy := range row {
				mark := ""
				if value, exists := strategiesMap[strategy.key]; exists && value == true {
					mark = "X"
				}

				// Celda con nombre de estrategia (80% del ancho)
				pdf.CellFormat(colWidth*0.8, 9, tr(strategy.label),
					"LTB", 0, "CM", false, 0, "")

				// Celda con marca X (20% del ancho)
				pdf.CellFormat(colWidth*0.2, 9, tr(mark),
					"LTRB", 0, "CM", false, 0, "")
			}
			pdf.Ln(-1) // Nueva línea después de cada fila
		}

	} else {
		FontStyle(pdf, "", 9, 0, "Helvetica")
		pdf.CellFormat(pageStyle.WC*10, 6, tr("No hay estrategias definidas"),
			"LBR", 1, "CM", false, 0, "")
	}
}

func drawEvaluationHeaders(pdf *gofpdf.Fpdf, tr func(string) string, colRA float64, colWidths []float64, evaluaciones []map[string]any) {
	FontStyle(pdf, "B", 7, 0, "Helvetica")

	startX, startY := pdf.GetXY()

	alturaFilaSuperior := 8.0 // Altura de la primera fila de encabezados
	alturaFilaInferior := 8.0 // Altura de la segunda fila de encabezados
	alturaTotal := alturaFilaSuperior + alturaFilaInferior

	// ===== PRIMERA COLUMNA: RA (ocupa ambas filas) =====
	pdf.Rect(startX, startY, colRA, alturaTotal, "D")

	// Centrar el texto verticalmente en la celda grande
	pdf.SetXY(startX+1, startY+alturaTotal/2-2) // Ajustar posición vertical
	pdf.MultiCell(colRA-2, 4, tr("Resultados de aprendizaje (RA) a ser evaluados"), "", "CM", false)

	// ===== SEGUNDA PARTE: Encabezados de evaluaciones =====

	// FILA SUPERIOR: "Resultados de aprendizaje asociados..."
	pdf.SetXY(startX+colRA, startY)
	totalWidthEvals := 0.0
	for _, width := range colWidths {
		totalWidthEvals += width
	}

	pdf.Rect(startX+colRA, startY, totalWidthEvals, alturaFilaSuperior, "D")
	pdf.SetXY(startX+colRA+1, startY+1)
	pdf.MultiCell(totalWidthEvals-2, 3, tr("Resultados de aprendizaje asociados a las evaluaciones\n(T: Teórico / P: Práctico)"), "", "CM", false)

	// FILA INFERIOR: Nombres específicos de evaluaciones
	currentX := startX + colRA
	currentY := startY + alturaFilaSuperior

	for i, eval := range evaluaciones {
		nombre := fmt.Sprintf("%v", eval["nombre"])

		// Simplificar nombres largos
		if len(nombre) > 12 {
			palabras := strings.Fields(nombre)
			if len(palabras) > 0 {
				nombre = palabras[0]
			}
		}

		pdf.Rect(currentX, currentY, colWidths[i], alturaFilaInferior, "D")
		pdf.SetXY(currentX+1, currentY+2)
		pdf.CellFormat(colWidths[i]-2, alturaFilaInferior-4, tr(nombre), "", 0, "CM", false, 0, "")

		currentX += colWidths[i]
	}

	pdf.SetXY(startX, startY+alturaTotal)
}

func evaluationSection(pdf *gofpdf.Fpdf, pageStyle PageStyle, data map[string]any) {
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.SetFillColor(pageStyle.BaseColorRGB[0], pageStyle.BaseColorRGB[1], pageStyle.BaseColorRGB[2])
	pdf.CellFormat(pageStyle.WW, 6, tr("Evaluación"), "LBR", 1, "CM", true, 0, "")

	// Obtener datos de evaluación
	evaluacionDet := data["evaluacion_detalle"]
	if evaluacionDet == nil {
		FontStyle(pdf, "", 9, 0, "Helvetica")
		pdf.CellFormat(pageStyle.WW, 6, tr("No hay detalles de evaluación definidos"), "LBR", 1, "CM", false, 0, "")
		return
	}

	// evaluaciones := evaluacionDet.([]any)

	descripcion_eval := data["evaluacion_descripcion"]
	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*10, 6, tr(" Descripción: "), "LR", 1, "LM", false, 0, "")
	FontStyle(pdf, "", 9, 0, "Helvetica")
	pdf.MultiCell(pageStyle.WC*10, 4.5, tr(fmt.Sprintf("%v\n", descripcion_eval)), "LR", "J", false)

	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*10, 6, tr("Evaluaciones: "), "LR", 1, "LM", false, 0, "")
	FontStyle(pdf, "", 9, 0, "Helvetica")

	evals := evaluacionDet.([]interface{})
	var texto_evaluaciones string
	for _, item := range evals {
		// // Totalmente automático, pero se genera sin orden, queda comentado por si sirve a futuro
		// pdf.MultiCell(pageStyle.WC*10, 4.5, tr("\t"), "LR", "J", false)
		// for key, value := range item.(map[string]interface{}) {
		// 	// pdf.MultiCell(pageStyle.WC*10, 4.5, tr(fmt.Sprintf("- %v: %v\n", key, value)), "LR", "J", false)
		// 	texto_evaluaciones += fmt.Sprintf(" %v: %v\n", key, value)
		// }

		if value, ok := item.(map[string]interface{}); ok {
			texto_evaluaciones += fmt.Sprintf(" - Nombre: %v\n", value["nombre"])
			texto_evaluaciones += fmt.Sprintf("\t   · Momento: %v\n", value["momento"])
			texto_evaluaciones += fmt.Sprintf("\t   · Estrategia: %v\n\n", value["estrategia"])
		}
	}
	pdf.MultiCell(pageStyle.WC*10, 4.5, tr(texto_evaluaciones), "LR", "J", false)

	// Espacio final
	pdf.CellFormat(pageStyle.WW, 2, "", "LBR", 1, "CM", false, 0, "")
}

func resourcesSection(pdf *gofpdf.Fpdf, pageStyle PageStyle, data map[string]any) {
	// MEDIOS Y RECURSOS EDUCATIVOS
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*10, 6, tr("Medios y recursos educativos"),
		"LBR", 1, "CM", true, 0, "")

	FontStyle(pdf, "", 9, 0, "Helvetica")
	resources, okResources := data["medios_recursos"]
	if okResources && resources != nil {
		pdf.MultiCell(pageStyle.WC*10, 4.5, tr(fmt.Sprintf("%v", resources)),
			"LBR", "J", false)
	} else {
		pdf.CellFormat(pageStyle.WC*10, 6, "", "LBR", 1, "LM", false, 0, "")
	}
}

func practicesSection(pdf *gofpdf.Fpdf, pageStyle PageStyle, data map[string]any) {
	// PRÁCTICAS ACADÉMICAS - SALIDAS DE CAMPO
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*10, 6, tr("Prácticas académicas"),
		"LBRT", 1, "CM", true, 0, "")

	FontStyle(pdf, "", 9, 0, "Helvetica")
	practices, okPractices := data["practicas_salidas"]
	if okPractices && practices != nil {
		pdf.MultiCell(pageStyle.WC*10, 4.5, tr(fmt.Sprintf("%v", practices)),
			"LBR", "J", false)
	} else {
		pdf.CellFormat(pageStyle.WC*10, 6, "", "LBR", 1, "LM", false, 0, "")
	}
}

func bibliographySection(pdf *gofpdf.Fpdf, pageStyle PageStyle, data map[string]any) {
	// BIBLIOGRAFÍA
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*10, 6, tr("Fuentes o referentes"),
		"LBRT", 1, "CM", true, 0, "")

	pdf.SetFillColor(255, 255, 255)
	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*10, 6, tr("Básicas"), "LR", 1, "LM", true, 0, "")
	FontStyle(pdf, "", 9, 0, "Helvetica")
	thematicDesc, okThematicDesc := data["bibliografia_basica"]
	if okThematicDesc && thematicDesc != nil {
		lista_bibliografia := ""
		for _, item := range thematicDesc.([]interface{}) {
			lista_bibliografia += fmt.Sprintf("- %v\n", item)
		}
		pdf.MultiCell(pageStyle.WC*10, 4.5, tr(fmt.Sprintf("%v \n\n", lista_bibliografia)),
			"LR", "J", false)
	} else {
		pdf.CellFormat(pageStyle.WC*10, 6, "", "LR", 1, "LM", false, 0, "")
	}

	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*10, 6, tr("Complementarias"), "LR", 1, "LM", true, 0, "")
	FontStyle(pdf, "", 9, 0, "Helvetica")
	thematicDesc, okThematicDesc = data["bibliografia_complementaria"]
	if okThematicDesc && thematicDesc != nil {
		lista_bibliografia := ""
		for _, item := range thematicDesc.([]interface{}) {
			lista_bibliografia += fmt.Sprintf("- %v\n", item)
		}
		pdf.MultiCell(pageStyle.WC*10, 4.5, tr(fmt.Sprintf("%v \n\n", lista_bibliografia)),
			"LR", "J", false)
	} else {
		pdf.CellFormat(pageStyle.WC*10, 6, "", "LR", 1, "LM", false, 0, "")
	}

	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*10, 6, tr("Bases de Datos"), "LR", 1, "LM", true, 0, "")
	FontStyle(pdf, "", 9, 0, "Helvetica")
	thematicDesc, okThematicDesc = data["bibliografia_bases"]
	if okThematicDesc && thematicDesc != nil {
		lista_bibliografia := ""
		for _, item := range thematicDesc.([]interface{}) {
			lista_bibliografia += fmt.Sprintf("- %v\n", item)
		}
		pdf.MultiCell(pageStyle.WC*10, 4.5, tr(fmt.Sprintf("%v \n\n", lista_bibliografia)),
			"LR", "J", false)
	} else {
		pdf.CellFormat(pageStyle.WC*10, 6, "", "LR", 1, "LM", false, 0, "")
	}

	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*10, 6, tr("Páginas web"), "LR", 1, "LM", true, 0, "")
	FontStyle(pdf, "", 9, 0, "Helvetica")
	thematicDesc, okThematicDesc = data["bibliografia_paginas"]
	if okThematicDesc && thematicDesc != nil {
		lista_bibliografia := ""
		for _, item := range thematicDesc.([]interface{}) {
			lista_bibliografia += fmt.Sprintf("- %v\n", item)
		}
		pdf.MultiCell(pageStyle.WC*10, 4.5, tr(fmt.Sprintf("%v \n\n", lista_bibliografia)),
			"LR", "J", false)
	} else {
		pdf.CellFormat(pageStyle.WC*10, 6, "", "LR", 1, "LM", false, 0, "")
	}

	pdf.SetFillColor(
		pageStyle.BaseColorRGB[0],
		pageStyle.BaseColorRGB[1],
		pageStyle.BaseColorRGB[2])
}

func trackingSection(pdf *gofpdf.Fpdf, pageStyle PageStyle, data map[string]any) {
	// SEGUIMIENTO Y ACTUALIZACIÓN DEL SYLLABUS
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*10, 6, tr("Seguimiento"),
		"LBRT", 1, "CM", true, 0, "")

	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC, 6, tr("Elaboró"),
		"LBR", 0, "LM", false, 0, "")
	FontStyle(pdf, "", 9, 0, "Helvetica")
	elaboro, elaboroOk := data["elaboro"]
	if !elaboroOk || elaboro == nil {
		elaboro = ""
	}
	pdf.CellFormat(pageStyle.WC*4, 6, tr(fmt.Sprintf("%v", elaboro)),
		"BR", 0, "CM", false, 0, "")
	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC, 6, tr("Fecha"),
		"BR", 0, "CM", false, 0, "")
	date_elaboro, date_elaboroOk := data["elaboro"]
	if !date_elaboroOk || date_elaboro == nil {
		date_elaboro = ""
	}
	FontStyle(pdf, "", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*4, 6, tr(fmt.Sprintf("%v", date_elaboro)),
		"BR", 1, "CM", false, 0, "")

	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC, 6, tr("Revisó"),
		"LBR", 0, "LM", false, 0, "")
	reviso, revisoOk := data["reviso"]
	if !revisoOk || reviso == nil {
		reviso = ""
	}
	FontStyle(pdf, "", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*4, 6, tr(fmt.Sprintf("%v", reviso)),
		"BR", 0, "CM", false, 0, "")
	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC, 6, tr("Fecha"),
		"BR", 0, "CM", false, 0, "")
	dateRev, dateRevOk := data["fecha_rev_consejo"]
	if !dateRevOk || dateRev == nil {
		dateRev = ""
	}
	FontStyle(pdf, "", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*4, 6, tr(fmt.Sprintf("%v", dateRev)),
		"BR", 1, "CM", false, 0, "")

	// FontStyle(pdf, "", 9, 0, "Helvetica")
	// pdf.CellFormat((pageStyle.WC*3)+2, 6, tr("Fecha revisión por Consejo Curricular:"),
	// 	"LBR", 0, "LM", false, 0, "")
	// dateRev, dateRevOk := data["fecha_rev_consejo"]
	// if !dateRevOk || dateRev == nil {
	// 	dateRev = ""
	// }
	// pdf.CellFormat((pageStyle.WC*2)-2, 6, tr(fmt.Sprintf("%v", dateRev)),
	// 	"BR", 0, "CM", false, 0, "")

	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC, 6, tr("Aprobó"),
		"LBR", 0, "LM", false, 0, "")
	FontStyle(pdf, "", 9, 0, "Helvetica")
	aprobo, aproboOk := data["aprobo"]
	if !aproboOk || aprobo == nil {
		aprobo = ""
	}
	pdf.CellFormat(pageStyle.WC*4, 6, tr(fmt.Sprintf("%v", aprobo)),
		"BR", 0, "CM", false, 0, "")
	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC, 6, tr("Fecha"),
		"BR", 0, "CM", false, 0, "")
	dateAprov, dateAprovOk := data["fecha_aprob_consejo"]
	if !dateAprovOk || dateAprov == nil {
		dateAprov = ""
	}
	FontStyle(pdf, "", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*4, 6, tr(fmt.Sprintf("%v", dateAprov)),
		"BR", 1, "CM", false, 0, "")

	pdf.CellFormat(pageStyle.WC*2, 6, tr("Número de acta:"), "LBR", 0, "LM", false, 0, "")
	numAct, numActOk := data["num_acta"]
	if !numActOk || numAct == nil {
		numAct = ""
	}
	pdf.CellFormat(pageStyle.WC*4, 6, tr(fmt.Sprintf("%v", numAct)), "BR", 0, "CM", false, 0, "")
	// pdf.MultiCell(pageStyle.WC*3, 6, tr(fmt.Sprintf("%v", numAct)), "LBR", "C", false)

	pdf.CellFormat(pageStyle.WC*2, 6, tr("Versión Syllabus:"), "LBR", 0, "LM", false, 0, "")
	versionSyll, versionSyllOk := data["version_syllabus"]
	if !versionSyllOk || versionSyll == nil {
		versionSyll = ""
	}
	pdf.CellFormat(pageStyle.WC*2, 6, tr(fmt.Sprintf("%v", versionSyll)),
		"BR", 1, "CM", false, 0, "")

	// pdf.CellFormat(pageStyle.WC*2, 6, tr("Documento versión: 12 julio 2023"),
	// 	"", 0, "LM", false, 0, "")
}

func mainSpaceData(pdf *gofpdf.Fpdf, pageStyle PageStyle, data map[string]any) {
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	headerTemplate(pdf, pageStyle)
	pdf.SetX(pageStyle.ML)

	// Sección Identificación institucional
	pdf.SetFillColor(
		pageStyle.BaseColorRGB[0],
		pageStyle.BaseColorRGB[1],
		pageStyle.BaseColorRGB[2])
	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*10, 6,
		tr("IDENTIFICACIÓN INSTITUCIONAL"),
		"LBRT", 1, "CM", true, 0, "")

	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*2, 6, tr("Facultad"), "LBR", 0, "LM", false, 0, "")
	faculty, facOk := data["nombre_facultad"]
	if !facOk || faculty == nil {
		faculty = ""
	}
	FontStyle(pdf, "", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*8, 6, tr(fmt.Sprintf("%v", faculty)),
		"LBR", 1, "LM", false, 0, "")

	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*2, 6, tr("Área de formación"), "LBR", 0, "LM", false, 0, "")
	area, areaOk := data["area_formacion"]
	if !areaOk || area == nil {
		area = ""
	}
	FontStyle(pdf, "", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*8, 6, tr(fmt.Sprintf("%v", area)),
		"LBR", 1, "LM", false, 0, "")

	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*2, 6, tr("Programa Académico"), "LBR", 0, "LM", false, 0, "")
	project, projectOk := data["nombre_proyecto_curricular"]
	if !projectOk || project == nil {
		project = ""
	}
	FontStyle(pdf, "", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*5, 6, tr(fmt.Sprintf("%v", project)),
		"LBR", 0, "LM", false, 0, "")

	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*2, 6, tr("Código plan de estudio"), "LBR", 0, "LM", false, 0, "")
	codPlan, codPlanOk := data["cod_plan_estudio"]
	if !codPlanOk || codPlan == nil {
		codPlan = ""
	}
	FontStyle(pdf, "", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC, 6, tr(fmt.Sprintf("%v", codPlan)),
		"LBR", 1, "CM", false, 0, "")
	pdf.CellFormat(pageStyle.WC*10, 3, tr(""),
		"LBR", 1, "CM", false, 0, "")

	pdf.SetFooterFunc(func() {
		tr := pdf.UnicodeTranslatorFromDescriptor("")
		pdf.SetY(-15)
		pdf.SetX(pageStyle.ML)
		pdf.SetFont("Helvetica", "I", 7)

		footerWidth := pageStyle.WC * 10

		pdf.MultiCell(footerWidth, 3, tr(`Estimada y estimado estudiante, si usted es una persona con discapacidad o cree que podría tener un problema dificultad o necesidad de aprendizaje y que requiere apoyos para cursar esta asignatura, por favor ponerse en contacto con el Centro Acacia al correo: cadep.acacia@udistrital.edu.co, o al teléfono: 601 3239300 Extensión 6213`),
			"", "C", false)
	})

	identificationSection(pdf, pageStyle, data)
	suggestionsSection(pdf, pageStyle, data)
	justificationSection(pdf, pageStyle, data)
	objectivesSection(pdf, pageStyle, data)
	purposeSection(pdf, pageStyle, data)
	thematicContentSection(pdf, pageStyle, data)
	// strategiesSection(pdf, pageStyle, data)
	evaluationSection(pdf, pageStyle, data)
	resourcesSection(pdf, pageStyle, data)
	practicesSection(pdf, pageStyle, data)
	bibliographySection(pdf, pageStyle, data)
	trackingSection(pdf, pageStyle, data)
}

func EncodePDF(pdf *gofpdf.Fpdf) string {
	var buffer bytes.Buffer
	writer := bufio.NewWriter(&buffer)
	pdf.Output(writer)
	writer.Flush()
	encodedFile := base64.StdEncoding.EncodeToString(buffer.Bytes())
	return encodedFile
}
func createTemplate(data map[string]any) (map[string]any, int) {
	pdf := gofpdf.New("P", "mm", "Letter", "")
	marginTB := 8.0
	marginLR := 8.0
	pdf.AddPage()
	pdf.SetMargins(marginLR, marginTB, marginLR)
	pdf.SetAutoPageBreak(true, 17)

	pageStyle := getPageStyle(pdf)
	pageStyle.WC = pageStyle.WW / 10

	mainSpaceData(pdf, pageStyle, data)

	if pdf.Err() {
		return map[string]any{
			"statusCode": 500,
			"body": map[string]any{
				"Success": false,
				"Status":  500,
				"Message": "Error error validating syllabus PDF!.",
			},
		}, 500
	}

	if pdf.Ok() {
		encodedFile := EncodePDF(pdf)
		return map[string]any{
			"statusCode": 200,
			"body": map[string]any{
				"Success": true,
				"Status":  200,
				"Message": "Syllabus Template OK",
				"Data":    encodedFile,
			},
		}, 200
	}

	return map[string]any{
		"statusCode": 500,
		"body": map[string]any{
			"Success": false,
			"Status":  500,
			"Message": "Error error generating syllabus!. Detail: ",
		},
	}, 500
}

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	var data map[string]any
	jsonErr := json.Unmarshal([]byte(request.Body), &data)
	if jsonErr != nil {
		//return events.APIGatewayProxyResponse{}, errors.New(jsonErr.Error())
		return events.APIGatewayProxyResponse{
			Body:       "Error getting data.",
			StatusCode: 500,
		}, nil
	}

	response, statusCode := createTemplate(data)
	jsonStr, err := json.Marshal(response)

	if err != nil {
		return events.APIGatewayProxyResponse{
			Body:       "Error generating json response.",
			StatusCode: 500,
		}, nil
	}

	return events.APIGatewayProxyResponse{
		Body:       string(jsonStr),
		StatusCode: statusCode,
	}, nil
}

func main() {
	lambda.Start(handler)
}
