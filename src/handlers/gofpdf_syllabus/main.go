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

func natureAcademicSpace(pdf *gofpdf.Fpdf, pageStyle PageStyle, data map[string]any) {
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	// Naturaleza del espacio académico
	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*10, 6, tr("NATURALEZA DEL ESPACIO ACADÉMICO (X):"),
		"LBR", 1, "CM", true, 0, "")

	FontStyle(pdf, "", 9, 0, "Helvetica")
	x, y := pdf.GetXY()
	pdf.MultiCell(pageStyle.WC, 6.75, tr("Obligatorio Básico"), "LBR", "CM", false)
	pdf.SetXY(x+pageStyle.WC, y)
	isOB := data["es_obligatorio_basico"]
	ob := ""
	if isOB == true {
		ob = "X"
	}
	pdf.CellFormat(pageStyle.WC, 13.5, tr(fmt.Sprintf("%v", ob)),
		"BR", 0, "CM", false, 0, "")

	pdf.MultiCell(pageStyle.WC, 4.5, tr("Obligatorio Comple-\nmentario"), "BR", "CM", false)
	pdf.SetXY(x+(pageStyle.WC*3), y)
	isOC := data["es_obligatorio_comp"]
	oc := ""
	if isOC == true {
		oc = "X"
	}
	pdf.CellFormat(pageStyle.WC, 13.5, tr(fmt.Sprintf("%v", oc)),
		"BR", 0, "CM", false, 0, "")

	pdf.MultiCell(pageStyle.WC, 6.75, tr("Electivo Intrínseco"), "BR", "CM", false)
	pdf.SetXY(x+(pageStyle.WC*5), y)
	isEI := data["es_electivo_int"]
	ei := ""
	if isEI == true {
		ei = "X"
	}
	pdf.CellFormat(pageStyle.WC, 13.5, tr(fmt.Sprintf("%v", ei)),
		"BR", 0, "CM", false, 0, "")

	pdf.MultiCell(pageStyle.WC, 6.75, tr("Electivo Extrínseco"),
		"BR", "CM", false)
	pdf.SetXY(x+(pageStyle.WC*7), y)
	isEE := data["es_electivo_ext"]
	ee := ""
	if isEE == true {
		ee = "X"
	}
	pdf.CellFormat(pageStyle.WC, 13.5, tr(fmt.Sprintf("%v", ee)),
		"BR", 0, "CM", false, 0, "")

	pdf.MultiCell(pageStyle.WC, 13.5, tr("Electivo"), "BR", "CM", false)
	pdf.SetXY(x+(pageStyle.WC*9), y)
	isE := data["es_electivo"]
	e := ""
	if isE == true {
		e = "X"
	}
	pdf.CellFormat(pageStyle.WC, 13.5, tr(fmt.Sprintf("%v", e)),
		"BR", 1, "CM", false, 0, "")
}

func characterAcademicSpace(pdf *gofpdf.Fpdf, pageStyle PageStyle, data map[string]any) {
	// Carácter del espacio académico
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*10, 6, tr("CARÁCTER DEL ESPACIO ACADÉMICO (X):"),
		"LRB", 1, "CM", true, 0, "")

	FontStyle(pdf, "", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*2, 6, tr("Teórico"), "LRB", 0, "CM", false, 0, "")
	isTheoretical := data["es_teorico"]
	theoretical := ""
	if isTheoretical == true {
		theoretical = "X"
	}
	pdf.CellFormat(pageStyle.WC, 6, tr(fmt.Sprintf("%v", theoretical)),
		"BR", 0, "CM", false, 0, "")

	pdf.CellFormat(pageStyle.WC*2, 6, tr("Práctico"), "RB", 0, "CM", false, 0, "")
	isPractical := data["es_practico"]
	practical := ""
	if isPractical == true {
		practical = "X"
	}
	pdf.CellFormat(pageStyle.WC, 6, tr(fmt.Sprintf("%v", practical)),
		"BR", 0, "CM", false, 0, "")

	pdf.CellFormat(pageStyle.WC*2, 6, tr("Teórico-Práctico"), "RB", 0, "CM", false, 0, "")
	isTheoreticalPractical := data["es_teorico_practico"]
	theoreticalPractical := ""
	if isTheoreticalPractical == true {
		theoreticalPractical = "X"
	}
	pdf.CellFormat(pageStyle.WC*2, 6, tr(fmt.Sprintf("%v", theoreticalPractical)),
		"BR", 1, "CM", false, 0, "")
}

func modalityAcademicSpace(pdf *gofpdf.Fpdf, pageStyle PageStyle, data map[string]any) {
	// Modalidad de oferta del espacio académico
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*10, 6, tr("MODALIDAD DE OFERTA DEL ESPACIO ACADÉMICO (X):"),
		"LRB", 1, "CM", true, 0, "")
	x, y := pdf.GetXY()

	FontStyle(pdf, "", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC, 18, tr("Presencial"), "LRB", 0, "CM", false, 0, "")
	isPresenceBased := data["es_presencial"]
	presenceBased := ""
	if isPresenceBased == true {
		presenceBased = "X"
	}
	pdf.CellFormat(pageStyle.WC, 18, tr(fmt.Sprintf("%v", presenceBased)),
		"BR", 0, "CM", false, 0, "")

	pdf.MultiCell(pageStyle.WC, 4.5, tr("Presencial con incorpo-\nración de TIC"),
		"BR", "CM", false)
	pdf.SetXY(x+(pageStyle.WC*3), y)
	isPresenceBasedTIC := data["es_presencial_tic"]
	presenceBasedTIC := ""
	if isPresenceBasedTIC == true {
		presenceBasedTIC = "X"
	}
	pdf.CellFormat(pageStyle.WC, 18, tr(fmt.Sprintf("%v", presenceBasedTIC)),
		"BR", 0, "CM", false, 0, "")

	pdf.CellFormat(pageStyle.WC, 18, tr("Virtual"), "LRB", 0, "CM", false, 0, "")
	isOnline := data["es_virtual"]
	online := ""
	if isOnline == true {
		online = "X"
	}
	pdf.CellFormat(pageStyle.WC, 18, tr(fmt.Sprintf("%v", online)),
		"BR", 0, "CM", false, 0, "")

	pdf.CellFormat(pageStyle.WC, 18, tr("Otros:"), "LRB", 0, "CM", false, 0, "")
	isOthersModality := data["otra_modalidad"]
	othersModality := ""
	if isOthersModality == true {
		othersModality = "X"
	}
	pdf.CellFormat(pageStyle.WC, 18, tr(fmt.Sprintf("%v", othersModality)),
		"BR", 0, "CM", false, 0, "")

	whichModality, okWhichModality := data["cual_otra_modalidad"]
	if okWhichModality && whichModality != nil {
		pdf.CellFormat(pageStyle.WC*2, 18, tr(fmt.Sprintf("Cuál: %v", whichModality)),
			"BR", 1, "CM", false, 0, "")
	} else {
		pdf.CellFormat(pageStyle.WC*2, 18, tr("Cuál:"), "BR", 1, "CM", false, 0, "")
	}
}

func languageAcademicSpace(pdf *gofpdf.Fpdf, pageStyle PageStyle, data map[string]any) {
	// Idioma
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*10, 6, tr("IDIOMA EN EL QUE SE OFERTA EL ESPACIO ACADÉMICO:"),
		"LRB", 1, "CM", true, 0, "")

	FontStyle(pdf, "", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*3, 6, tr("Idioma"), "LRB", 0, "CM", false, 0, "")
	language, okLanguage := data["idiomas"]
	if okLanguage && language != nil {
		pdf.CellFormat(pageStyle.WC*7, 6, tr(fmt.Sprintf("%v", language)),
			"BR", 1, "LM", false, 0, "")
	} else {
		pdf.CellFormat(pageStyle.WC*7, 6, tr(""), "BR", 1, "LM", false, 0, "")
	}
}

func identificationSection(pdf *gofpdf.Fpdf, pageStyle PageStyle, data map[string]any) {
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	pdf.SetX(pageStyle.ML)
	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*10, 6, tr("I. IDENTIFICACIÓN DEL ESPACIO ACADÉMICO"),
		"LBR", 1, "CM", false, 0, "")

	spaceName, spaceNameOk := data["nombre_espacio_academico"]
	if !spaceNameOk || spaceName == nil {
		spaceName = ""
	}
	pdf.SetFillColor(
		pageStyle.BaseColorRGB[0],
		pageStyle.BaseColorRGB[1],
		pageStyle.BaseColorRGB[2])
	pdf.CellFormat(pageStyle.WC*10, 6,
		tr(fmt.Sprintf("NOMBRE DEL ESPACIO ACADÉMICO: %v", spaceName)),
		"LBR", 1, "LM", true, 0, "")

	// Código del espacio académico
	FontStyle(pdf, "", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*4, 6, tr("Código del espacio académico:"),
		"LBR", 0, "LM", false, 0, "")
	spaceCod, spaceCodOk := data["cod_espacio_academico"]
	if !spaceCodOk || spaceCod == nil {
		spaceCod = ""
	}
	pdf.CellFormat(pageStyle.WC, 6, tr(fmt.Sprintf("%v", spaceCod)),
		"BR", 0, "CM", false, 0, "")

	pdf.CellFormat(pageStyle.WC*3, 6, tr("Número de créditos académicos:"),
		"BR", 0, "LM", false, 0, "")
	numCredits, numCreditsOk := data["num_creditos"]
	if !numCreditsOk || numCredits == nil {
		numCredits = ""
	}
	pdf.CellFormat(pageStyle.WC*2, 6, tr(fmt.Sprintf("%v", numCredits)),
		"BR", 1, "CM", false, 0, "")

	//	Distribución horas de trabajo
	pdf.CellFormat(pageStyle.WC*4, 6, tr("Distribución horas de trabajo:"),
		"LBR", 0, "LM", false, 0, "")
	pdf.CellFormat(pageStyle.WC, 6, "HTD", "BR", 0, "CM", false, 0, "")
	htd, htdOk := data["htd"]
	if !htdOk || htd == nil {
		htd = ""
	}
	pdf.CellFormat(pageStyle.WC, 6, tr(fmt.Sprintf("%v", htd)),
		"BR", 0, "CM", false, 0, "")
	pdf.CellFormat(pageStyle.WC, 6, "HTC", "BR", 0, "CM", false, 0, "")
	htc, htcOk := data["htc"]
	if !htcOk || htc == nil {
		htc = ""
	}
	pdf.CellFormat(pageStyle.WC, 6, tr(fmt.Sprintf("%v", htc)),
		"BR", 0, "CM", false, 0, "")
	pdf.CellFormat(pageStyle.WC, 6, "HTA", "BR", 0, "CM", false, 0, "")
	hta, htaOk := data["hta"]
	if !htaOk || hta == nil {
		hta = ""
	}
	pdf.CellFormat(pageStyle.WC, 6, tr(fmt.Sprintf("%v", hta)),
		"BR", 1, "CM", false, 0, "")

	//	Tipo de espacio académico
	pdf.CellFormat(pageStyle.WC*4, 6, tr("Tipo de espacio académico (X):"),
		"LBR", 0, "LM", false, 0, "")
	pdf.CellFormat(pageStyle.WC, 6, "Asignatura", "BR", 0, "CM", false, 0, "")
	isCourse := data["es_asignatura"]
	course := ""
	if isCourse == true {
		course = "X"
	}
	pdf.CellFormat(pageStyle.WC, 6, tr(fmt.Sprintf("%v", course)),
		"BR", 0, "CM", false, 0, "")
	pdf.CellFormat(pageStyle.WC, 6, tr("Cátedra"), "BR", 0, "CM", false, 0, "")
	isChair := data["es_catedra"]
	chair := ""
	if isChair == true {
		chair = "X"
	}
	pdf.CellFormat(pageStyle.WC, 6, tr(fmt.Sprintf("%v", chair)),
		"BR", 0, "CM", false, 0, "")
	pdf.CellFormat(pageStyle.WC, 6, "", "BR", 0, "CM", false, 0, "")
	pdf.CellFormat(pageStyle.WC, 6, "", "BR", 1, "CM", false, 0, "")

	natureAcademicSpace(pdf, pageStyle, data)
	characterAcademicSpace(pdf, pageStyle, data)
	modalityAcademicSpace(pdf, pageStyle, data)
	languageAcademicSpace(pdf, pageStyle, data)
}

func suggestionsSection(pdf *gofpdf.Fpdf, pageStyle PageStyle, data map[string]any) {
	// SUGERENCIAS DE SABERES Y CONOCIMIENTOS PREVIOS
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*10, 6, tr("II. SUGERENCIAS DE SABERES Y CONOCIMIENTOS PREVIOS"),
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
	pdf.CellFormat(pageStyle.WC*10, 6, tr("III. JUSTIFICACIÓN DEL ESPACIO ACADÉMICO"),
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
		tr("IV. OBJETIVOS DEL ESPACIO ACADÉMICO (GENERAL Y ESPECÍFICOS)"),
		"LRB", 1, "CM", true, 0, "")

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
	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.SetFillColor(pageStyle.BaseColorRGB[0], pageStyle.BaseColorRGB[1], pageStyle.BaseColorRGB[2])
	pdf.CellFormat(pageStyle.WW, 6, // Usando pageStyle.WW para el ancho total
		tr("V. PROPÓSITOS DE FORMACIÓN Y DE APRENDIZAJE (PFA) DEL ESPACIO ACADÉMICO"),
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
	pdf.CellFormat(pageStyle.WC*10, 6, tr("VI. CONTENIDOS TEMÁTICOS"),
		"LRB", 1, "CM", true, 0, "")
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
	pdf.CellFormat(pageStyle.WW, 6, tr("VIII. EVALUACIÓN"), "LRB", 1, "CM", true, 0, "")

	// Obtener datos de evaluación
	evaluacionDet := data["evaluacion_detalle"]
	if evaluacionDet == nil {
		FontStyle(pdf, "", 9, 0, "Helvetica")
		pdf.CellFormat(pageStyle.WW, 6, tr("No hay detalles de evaluación definidos"), "LBR", 1, "CM", false, 0, "")
		return
	}

	evaluaciones := evaluacionDet.([]any)

	descripcion_eval := data["evaluacion_descripcion"]
	if descripcion_eval.(string) != "" {
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

	} else {
		numEvaluaciones := len(evaluaciones)

		// Configurar anchos de columnas
		colRA := pageStyle.WW * 0.25          // 25% para columna de RAs
		remainingWidth := pageStyle.WW * 0.75 // 75% para las evaluaciones
		colEvalWidth := remainingWidth / float64(numEvaluaciones)

		// Array de anchos para cada columna de evaluación
		colWidths := make([]float64, numEvaluaciones)
		for i := range colWidths {
			colWidths[i] = colEvalWidth
		}

		// Convertir evaluaciones a slice de maps
		evaluacionesMaps := make([]map[string]any, 0)
		for _, eval := range evaluaciones {
			evaluacionesMaps = append(evaluacionesMaps, eval.(map[string]any))
		}

		// Obtener todos los RAs únicos y ordenarlos
		rasSet := make(map[string]bool)
		for _, eval := range evaluacionesMaps {
			if rasAsociados, ok := eval["resultados_aprendizaje_asociados"]; ok && rasAsociados != nil {
				for _, ra := range rasAsociados.([]any) {
					rasSet[fmt.Sprintf("%v", ra)] = true
				}
			}
		}

		// Convertir a slice ordenado (01, 02, 03, etc.)
		ras := make([]string, 0)
		for i := 1; i <= 9; i++ {
			raStr := fmt.Sprintf("%02d", i)
			if rasSet[raStr] {
				ras = append(ras, raStr)
			}
		}

		_, pageH := pdf.GetPageSize()
		const lineHeight = 4.5

		// Dibujar encabezados
		drawEvaluationHeaders(pdf, tr, colRA, colWidths, evaluacionesMaps)

		// Calcular altura total de la tabla
		numFilas := len(ras) + 4                          // RAs + 4 filas de metadatos
		alturaTabla := float64(numFilas) * lineHeight * 2 // Estimación generosa

		// Verificar si cabe en la página
		if pdf.GetY()+alturaTabla > (pageH - pageStyle.MB) {
			pdf.AddPage()
			drawEvaluationHeaders(pdf, tr, colRA, colWidths, evaluacionesMaps)
		}

		// Filas de RAs
		for _, ra := range ras {
			currentY := pdf.GetY()

			// Verificar si necesitamos nueva página
			if currentY+lineHeight*2 > (pageH - pageStyle.MB) {
				pdf.AddPage()
				drawEvaluationHeaders(pdf, tr, colRA, colWidths, evaluacionesMaps)
			}

			FontStyle(pdf, "", 8, 0, "Helvetica")
			alturaCelda := lineHeight * 2

			// Columna de RA
			pdf.CellFormat(colRA, alturaCelda, tr(fmt.Sprintf("RA%s", ra)), "LRB", 0, "CM", false, 0, "")

			// Columnas de evaluaciones - marcar con X si el RA está asociado
			for i, eval := range evaluacionesMaps {
				marca := ""
				if rasAsociados, ok := eval["resultados_aprendizaje_asociados"]; ok && rasAsociados != nil {
					for _, raAsociado := range rasAsociados.([]any) {
						if fmt.Sprintf("%v", raAsociado) == ra {
							marca = "X"
							break
						}
					}
				}
				pdf.CellFormat(colWidths[i], alturaCelda, tr(marca), "RB", 0, "CM", false, 0, "")
			}
			pdf.Ln(-1)
		}

		// Filas de metadatos
		metadatos := []struct {
			nombre string
			campo  string
		}{
			{"Tipo de evaluación", "tipo_evaluacion"},
			{"Porcentaje de evaluación (%)", "porcentaje"},
			{"Trabajo Individual(I) o Grupal(G)", "trabajo_tipo"},
			{"Tipo de nota", "tipo_nota"},
		}

		for _, meta := range metadatos {
			currentY := pdf.GetY()

			// Verificar si necesitamos nueva página
			if currentY+lineHeight*2 > (pageH - pageStyle.MB) {
				pdf.AddPage()
				drawEvaluationHeaders(pdf, tr, colRA, colWidths, evaluacionesMaps)
			}

			FontStyle(pdf, "B", 8, 0, "Helvetica")
			alturaCelda := lineHeight * 2

			// Columna de metadato
			pdf.CellFormat(colRA, alturaCelda, tr(meta.nombre), "LRB", 0, "LM", false, 0, "")

			// Valores para cada evaluación
			FontStyle(pdf, "", 8, 0, "Helvetica")
			for i, eval := range evaluacionesMaps {
				valor := ""
				if val, ok := eval[meta.campo]; ok && val != nil {
					valor = fmt.Sprintf("%v", val)
				}
				pdf.CellFormat(colWidths[i], alturaCelda, tr(valor), "RB", 0, "CM", false, 0, "")
			}
			pdf.Ln(-1)
		}
	}
	// Espacio final
	pdf.CellFormat(pageStyle.WW, 2, "", "LBR", 1, "CM", false, 0, "")
}

func resourcesSection(pdf *gofpdf.Fpdf, pageStyle PageStyle, data map[string]any) {
	// MEDIOS Y RECURSOS EDUCATIVOS
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*10, 6, tr("IX. MEDIOS Y RECURSOS EDUCATIVOS"),
		"LRB", 1, "CM", true, 0, "")

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
	pdf.CellFormat(pageStyle.WC*10, 6, tr("X. PRÁCTICAS ACADÉMICAS - SALIDAS DE CAMPO"),
		"LRB", 1, "CM", true, 0, "")

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
	pdf.CellFormat(pageStyle.WC*10, 6, tr("XI. BIBLIOGRAFÍA"),
		"LRB", 1, "CM", true, 0, "")

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
	pdf.CellFormat(pageStyle.WC*10, 6, tr("XII. SEGUIMIENTO Y ACTUALIZACIÓN DEL SYLLABUS"),
		"LRB", 1, "CM", true, 0, "")

	FontStyle(pdf, "", 9, 0, "Helvetica")
	pdf.CellFormat((pageStyle.WC*3)+2, 6, tr("Fecha revisión por Consejo Curricular:"),
		"LBR", 0, "LM", false, 0, "")
	dateRev, dateRevOk := data["fecha_rev_consejo"]
	if !dateRevOk || dateRev == nil {
		dateRev = ""
	}
	pdf.CellFormat((pageStyle.WC*2)-2, 6, tr(fmt.Sprintf("%v", dateRev)),
		"BR", 0, "CM", false, 0, "")

	pdf.CellFormat(pageStyle.WC*2, 6, tr("Versión Syllabus:"), "LBR", 0, "LM", false, 0, "")
	versionSyll, versionSyllOk := data["version_syllabus"]
	if !versionSyllOk || versionSyll == nil {
		versionSyll = ""
	}
	pdf.CellFormat(pageStyle.WC*3, 6, tr(fmt.Sprintf("%v", versionSyll)),
		"BR", 1, "CM", false, 0, "")

	pdf.CellFormat((pageStyle.WC*3)+2, 6, tr("Fecha aprobación por Consejo Curricular:"),
		"LBR", 0, "LM", false, 0, "")
	dateAprov, dateAprovOk := data["fecha_aprob_consejo"]
	if !dateAprovOk || dateAprov == nil {
		dateAprov = ""
	}
	pdf.CellFormat((pageStyle.WC*2)-2, 6, tr(fmt.Sprintf("%v", dateAprov)),
		"BR", 0, "CM", false, 0, "")

	pdf.CellFormat(pageStyle.WC*2, 6, tr("Número de acta:"), "LBR", 0, "LM", false, 0, "")
	numAct, numActOk := data["num_acta"]
	if !numActOk || numAct == nil {
		numAct = ""
	}
	// pdf.CellFormat(pageStyle.WC*3, 6, tr(fmt.Sprintf("%v", numAct)), "BR", 1, "CM", false, 0, "")
	pdf.MultiCell(pageStyle.WC*3, 6, tr(fmt.Sprintf("%v", numAct)), "LBR", "C", false)

	pdf.CellFormat(pageStyle.WC*3, 6, tr("Documento versión: 12 julio 2023"),
		"", 0, "LM", false, 0, "")
}

func mainSpaceData(pdf *gofpdf.Fpdf, pageStyle PageStyle, data map[string]any) {
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	headerTemplate(pdf, pageStyle)
	pdf.SetX(pageStyle.ML)
	pdf.CellFormat(pageStyle.WC*2, 6, tr("FACULTAD:"), "LBR", 0, "LM", false, 0, "")
	FontStyle(pdf, "", 9, 0, "Helvetica")
	faculty, facOk := data["nombre_facultad"]
	if !facOk || faculty == nil {
		faculty = ""
	}
	pdf.CellFormat(pageStyle.WC*8, 6, tr(fmt.Sprintf("%v", faculty)),
		"LBR", 1, "LM", false, 0, "")

	FontStyle(pdf, "B", 9, 0, "Helvetica")
	x, y := pdf.GetXY()
	pdf.MultiCell(pageStyle.WC*2, 4.5, tr("PROYECTO \nCURRICULAR:"), "LBR", "LM", false)
	pdf.SetXY(x+pageStyle.WC*2, y)
	project, projectOk := data["nombre_proyecto_curricular"]
	if !projectOk || project == nil {
		project = ""
	}
	FontStyle(pdf, "", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*4, 9, tr(fmt.Sprintf("%v", project)),
		"LBR", 0, "LM", false, 0, "")

	FontStyle(pdf, "B", 9, 0, "Helvetica")
	pdf.MultiCell(pageStyle.WC*2, 4.5, tr("CÓDIGO PLAN DE \nESTUDIOS:"), "LBR", "LM", false)
	pdf.SetXY(x+pageStyle.WC*8, y)
	codPlan, codPlanOk := data["cod_plan_estudio"]
	if !codPlanOk || codPlan == nil {
		codPlan = ""
	}
	FontStyle(pdf, "", 9, 0, "Helvetica")
	pdf.CellFormat(pageStyle.WC*2, 9, tr(fmt.Sprintf("%v", codPlan)),
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
	strategiesSection(pdf, pageStyle, data)
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
