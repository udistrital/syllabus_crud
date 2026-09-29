package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	excelize "github.com/xuri/excelize/v2"
)

func templateStyle(template excelize.File) map[string]int {
	// Define styles
	boldStyle, _ := template.NewStyle(
		&excelize.Style{
			Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
			Font:      &excelize.Font{Size: 9, Bold: true},
			Border: []excelize.Border{
				{Type: "right", Color: "000000", Style: 1},
				{Type: "left", Color: "000000", Style: 1},
				{Type: "top", Color: "000000", Style: 1},
				{Type: "bottom", Color: "000000", Style: 1},
			},
		})
	boldLeftStyle, _ := template.NewStyle(
		&excelize.Style{
			Alignment: &excelize.Alignment{
				Horizontal: "left", Vertical: "center", WrapText: true},
			Font: &excelize.Font{Size: 9, Bold: true},
			Border: []excelize.Border{
				{Type: "right", Color: "000000", Style: 1},
				{Type: "left", Color: "000000", Style: 1},
				{Type: "top", Color: "000000", Style: 1},
				{Type: "bottom", Color: "FFFFFF", Style: 1},
			},
		})
	boldLeftLTRStyle, _ := template.NewStyle(
		&excelize.Style{
			Alignment: &excelize.Alignment{
				Horizontal: "left", Vertical: "center", WrapText: true},
			Font: &excelize.Font{Size: 9, Bold: true},
			Border: []excelize.Border{
				{Type: "right", Color: "000000", Style: 1},
				{Type: "left", Color: "000000", Style: 1},
				{Type: "top", Color: "000000", Style: 1},
				{Type: "bottom", Color: "FFFFFF", Style: 0},
			},
		})
	boldLeftLRStyle, _ := template.NewStyle(
		&excelize.Style{
			Alignment: &excelize.Alignment{
				Horizontal: "left", Vertical: "center", WrapText: true},
			Font: &excelize.Font{Size: 9, Bold: true},
			Border: []excelize.Border{
				{Type: "right", Color: "000000", Style: 1},
				{Type: "left", Color: "000000", Style: 1},
				{Type: "top", Color: "FFFFFF", Style: 1},
				{Type: "bottom", Color: "FFFFFF", Style: 1},
			},
		})
	boldFillStyle, _ := template.NewStyle(
		&excelize.Style{
			Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
			Font:      &excelize.Font{Size: 9, Bold: true},
			Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"c9c9c9"}},
			Border: []excelize.Border{
				{Type: "right", Color: "000000", Style: 1},
				{Type: "left", Color: "000000", Style: 1},
				{Type: "top", Color: "000000", Style: 1},
				{Type: "bottom", Color: "000000", Style: 1},
			},
		})
	boldFillLeftStyle, _ := template.NewStyle(
		&excelize.Style{
			Alignment: &excelize.Alignment{
				Horizontal: "left", Vertical: "center", WrapText: true},
			Font: &excelize.Font{Size: 9, Bold: true},
			Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"c9c9c9"}},
			Border: []excelize.Border{
				{Type: "right", Color: "000000", Style: 1},
				{Type: "left", Color: "000000", Style: 1},
				{Type: "top", Color: "000000", Style: 1},
				{Type: "bottom", Color: "000000", Style: 1},
			},
		})
	leftStyle, _ := template.NewStyle(
		&excelize.Style{
			Alignment: &excelize.Alignment{
				Horizontal: "left", Vertical: "center", WrapText: true},
			Font: &excelize.Font{Size: 9, Bold: false},
			Border: []excelize.Border{
				{Type: "right", Color: "000000", Style: 0},
				{Type: "left", Color: "000000", Style: 0},
				{Type: "top", Color: "000000", Style: 0},
				{Type: "bottom", Color: "000000", Style: 0},
			},
		})
	simpleStyle, _ := template.NewStyle(
		&excelize.Style{
			Alignment: &excelize.Alignment{
				Horizontal: "center", Vertical: "center", WrapText: true},
			Font: &excelize.Font{Size: 9, Bold: false},
			Border: []excelize.Border{
				{Type: "right", Color: "000000", Style: 1},
				{Type: "left", Color: "000000", Style: 1},
				{Type: "top", Color: "000000", Style: 1},
				{Type: "bottom", Color: "000000", Style: 1},
			},
		})
	simpleLeftStyle, _ := template.NewStyle(
		&excelize.Style{
			Alignment: &excelize.Alignment{
				Horizontal: "left", Vertical: "center", WrapText: true},
			Font: &excelize.Font{Size: 9, Bold: false},
			Border: []excelize.Border{
				{Type: "right", Color: "000000", Style: 1},
				{Type: "left", Color: "000000", Style: 1},
				{Type: "top", Color: "000000", Style: 1},
				{Type: "bottom", Color: "000000", Style: 1},
			},
		})
	simpleJustifyStyle, _ := template.NewStyle(
		&excelize.Style{
			Alignment: &excelize.Alignment{
				Horizontal: "justify", Vertical: "center", WrapText: true},
			Font: &excelize.Font{Size: 9, Bold: false},
			Border: []excelize.Border{
				{Type: "right", Color: "000000", Style: 1},
				{Type: "left", Color: "000000", Style: 1},
				{Type: "top", Color: "000000", Style: 1},
				{Type: "bottom", Color: "000000", Style: 1},
			},
		})
	simpleJustifyLRStyle, _ := template.NewStyle(
		&excelize.Style{
			Alignment: &excelize.Alignment{
				Horizontal: "justify", Vertical: "center", WrapText: true},
			Font: &excelize.Font{Size: 9, Bold: false},
			Border: []excelize.Border{
				{Type: "right", Color: "000000", Style: 1},
				{Type: "left", Color: "000000", Style: 1},
				{Type: "top", Color: "FFFFFF", Style: 1},
				{Type: "bottom", Color: "FFFFFF", Style: 1},
			},
		})

	styles := map[string]int{
		"boldStyle":            boldStyle,
		"boldLeftStyle":        boldLeftStyle,
		"boldLeftLTRStyle":     boldLeftLTRStyle,
		"boldLeftLRStyle":      boldLeftLRStyle,
		"boldFillStyle":        boldFillStyle,
		"boldFillLeftStyle":    boldFillLeftStyle,
		"leftStyle":            leftStyle,
		"simpleStyle":          simpleStyle,
		"simpleLeftStyle":      simpleLeftStyle,
		"simpleJustifyStyle":   simpleJustifyStyle,
		"simpleJustifyLRStyle": simpleJustifyLRStyle,
	}
	return styles
}

const (
	hRow  = 17.0
	hTall = 34.0
	hSep  = 9.0
)

func colLetter(n int) string {
	return string(rune('A' + n - 1))
}

func cellRef(col, row int) string {
	return fmt.Sprintf("%s%d", colLetter(col), row)
}

func getString(data map[string]any, key string) string {
	v, ok := data[key]
	if !ok || v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

func mergeSet(t *excelize.File, sheet string, c1, c2, row int, styleKey, value string, styles map[string]int, height float64) {
	start := cellRef(c1, row)
	end := cellRef(c2, row)
	if c1 != c2 {
		t.MergeCell(sheet, start, end)
	}
	t.SetCellStyle(sheet, start, end, styles[styleKey])
	if value != "" {
		t.SetCellValue(sheet, start, value)
	}
	if height > 0 {
		t.SetRowHeight(sheet, row, height)
	}
}

func addHeaderImage(t *excelize.File, sheet, anchor string, data []byte, ext string, maxW, maxH float64, offsetX, offsetY int) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width == 0 {
		fmt.Println("Error leyendo dimensiones de imagen:", err)
		return
	}
	scale := maxW / float64(cfg.Width)
	if s := maxH / float64(cfg.Height); s < scale {
		scale = s
	}
	if err := t.AddPictureFromBytes(sheet, anchor, &excelize.Picture{
		Extension: ext,
		File:      data,
		Format: &excelize.GraphicOptions{
			AltText: "logo",
			ScaleX:  scale,
			ScaleY:  scale,
			OffsetX: offsetX,
			OffsetY: offsetY,
		},
	}); err != nil {
		fmt.Println("Error agregando imagen:", err)
	}
}

func drawHeaderImages(t *excelize.File, sheet string) {
	// Ajuste visual: la izquierda ocupa ~2 columnas, la derecha se alinea a la derecha de H:J
	addHeaderImage(t, sheet, "A1", logoIzquierdo, ".png", 150, 140, 35, 2)
	addHeaderImage(t, sheet, "H1", logoSigud, ".jpg", 200, 90, 45, 40)
}

func createHeader(template *excelize.File, sheetName string, style map[string]int) {
	// Una columna = 1 WC; ancho uniforme A..J
	for c := 1; c <= 10; c++ {
		col := colLetter(c)
		template.SetColWidth(sheetName, col, col, 11)
	}

	// Header en 3 filas con alto proporcional 1:1:2
	template.SetRowHeight(sheetName, 1, 28)
	template.SetRowHeight(sheetName, 2, 28)
	template.SetRowHeight(sheetName, 3, 57)

	// Bandas: A-B (logo izq) | C-E | F-G | H-J (logo der)
	template.MergeCell(sheetName, "A1", "B3")
	template.MergeCell(sheetName, "H1", "J3")
	template.MergeCell(sheetName, "C1", "E1")
	template.MergeCell(sheetName, "C2", "E2")
	template.MergeCell(sheetName, "C3", "E3")
	template.MergeCell(sheetName, "F1", "G1")
	template.MergeCell(sheetName, "F2", "G2")
	template.MergeCell(sheetName, "F3", "G3")

	// Bordes y estilos
	template.SetCellStyle(sheetName, "A1", "J3", style["simpleStyle"])
	template.SetCellStyle(sheetName, "C1", "E1", style["boldStyle"])
	template.SetCellStyle(sheetName, "C2", "E2", style["simpleStyle"])
	template.SetCellStyle(sheetName, "C3", "E3", style["simpleStyle"])
	template.SetCellStyle(sheetName, "F1", "G1", style["simpleStyle"])
	template.SetCellStyle(sheetName, "F2", "G2", style["simpleStyle"])
	template.SetCellStyle(sheetName, "F3", "G3", style["simpleStyle"])

	// Textos (mismos del PDF)
	template.SetCellValue(sheetName, "C1", "FORMATO DE SYLLABUS")
	template.SetCellValue(sheetName, "F1", "Código: CC-FR-003")
	template.SetCellValue(sheetName, "C2", "Macroproceso: Direccionamiento Estratégico")
	template.SetCellValue(sheetName, "F2", "Versión: 02")
	template.SetCellValue(sheetName, "C3", "Proceso: Currículo y Calidad")
	template.SetCellValue(sheetName, "F3", "Fecha de Aprobación: XX-XX-2026")

	drawHeaderImages(template, sheetName)
}

func institutionalSection(template *excelize.File, sheetName string, style map[string]int, data map[string]any, index *int) {
	mergeSet(template, sheetName, 1, 10, *index, "boldFillStyle", "IDENTIFICACIÓN INSTITUCIONAL", style, hRow)
	*index++

	mergeSet(template, sheetName, 1, 2, *index, "boldLeftStyle", "Facultad", style, hRow)
	mergeSet(template, sheetName, 3, 10, *index, "simpleLeftStyle", getString(data, "nombre_facultad"), style, hRow)
	*index++

	mergeSet(template, sheetName, 1, 2, *index, "boldLeftStyle", "Área de formación", style, hRow)
	mergeSet(template, sheetName, 3, 10, *index, "simpleLeftStyle", getString(data, "area_formacion"), style, hRow)
	*index++

	mergeSet(template, sheetName, 1, 2, *index, "boldLeftStyle", "Programa Académico", style, hRow)
	mergeSet(template, sheetName, 3, 7, *index, "simpleLeftStyle", getString(data, "nombre_proyecto_curricular"), style, hRow)
	mergeSet(template, sheetName, 8, 9, *index, "boldLeftStyle", "Código plan de estudio", style, hRow)
	mergeSet(template, sheetName, 10, 10, *index, "simpleStyle", getString(data, "cod_plan_estudio"), style, hRow)
	*index++

	mergeSet(template, sheetName, 1, 10, *index, "simpleStyle", "", style, hSep)
	*index++
}

func identificationSection(template *excelize.File, sheetName string, style map[string]int, data map[string]any, index *int) {
	mergeSet(template, sheetName, 1, 10, *index, "boldFillStyle", "IDENTIFICACIÓN DEL ESPACIO ACADÉMICO", style, hRow)
	*index++

	// Nombre del espacio academico | valor | Idioma | valor (2 lineas)
	mergeSet(template, sheetName, 1, 3, *index, "boldStyle", "Nombre del espacio académico", style, hTall)
	mergeSet(template, sheetName, 4, 6, *index, "simpleStyle", getString(data, "nombre_espacio_academico"), style, hTall)
	mergeSet(template, sheetName, 7, 8, *index, "boldStyle", "Idioma", style, hTall)
	mergeSet(template, sheetName, 9, 10, *index, "simpleStyle", getString(data, "idiomas"), style, hTall)
	*index++

	// Codigo | Creditos | Modalidad
	mergeSet(template, sheetName, 1, 3, *index, "boldLeftStyle", "Código del espacio académico", style, hRow)
	mergeSet(template, sheetName, 4, 4, *index, "simpleStyle", getString(data, "cod_espacio_academico"), style, hRow)
	mergeSet(template, sheetName, 5, 7, *index, "boldLeftStyle", "Número de créditos académicos", style, hRow)
	mergeSet(template, sheetName, 8, 8, *index, "simpleStyle", getString(data, "num_creditos"), style, hRow)
	mergeSet(template, sheetName, 9, 9, *index, "boldLeftStyle", "Modalidad", style, hRow)
	mergeSet(template, sheetName, 10, 10, *index, "simpleStyle", getString(data, "modalidad"), style, hRow)
	*index++

	// Distribucion horas de trabajo
	mergeSet(template, sheetName, 1, 4, *index, "boldLeftStyle", "Distribución horas de trabajo", style, hRow)
	mergeSet(template, sheetName, 5, 5, *index, "boldStyle", "HTD", style, hRow)
	mergeSet(template, sheetName, 6, 6, *index, "simpleStyle", getString(data, "htd"), style, hRow)
	mergeSet(template, sheetName, 7, 7, *index, "boldStyle", "HTC", style, hRow)
	mergeSet(template, sheetName, 8, 8, *index, "simpleStyle", getString(data, "htc"), style, hRow)
	mergeSet(template, sheetName, 9, 9, *index, "boldStyle", "HTA", style, hRow)
	mergeSet(template, sheetName, 10, 10, *index, "simpleStyle", getString(data, "hta"), style, hRow)
	*index++

	// Clasificacion (naturaleza) | Caracter
	mergeSet(template, sheetName, 1, 3, *index, "boldLeftStyle", "Clasificación del espacio académico", style, hRow)
	mergeSet(template, sheetName, 4, 5, *index, "simpleStyle", getString(data, "naturaleza"), style, hRow)
	mergeSet(template, sheetName, 6, 8, *index, "boldLeftStyle", "Carácter del espacio académico", style, hRow)
	mergeSet(template, sheetName, 9, 10, *index, "simpleStyle", getString(data, "caracter"), style, hRow)
	*index++

	mergeSet(template, sheetName, 1, 10, *index, "simpleStyle", "", style, hSep)
	*index++
}

func suggestionsSection(template *excelize.File, sheetName string, style map[string]int, data map[string]any, index *int) {
	mergeSet(template, sheetName, 1, 10, *index, "boldFillStyle", "Saberes y conocimientos previos", style, hRow)
	*index++
	suggestions, okSuggestions := data["sugerencias"]
	if okSuggestions && suggestions != nil {
		mergeSet(template, sheetName, 1, 10, *index, "simpleJustifyStyle", fmt.Sprintf("%v", suggestions), style, 50)
	} else {
		mergeSet(template, sheetName, 1, 10, *index, "simpleJustifyStyle", "", style, hRow)
	}
	*index++
}

func justificationSection(template *excelize.File, sheetName string, style map[string]int, data map[string]any, index *int) {
	mergeSet(template, sheetName, 1, 10, *index, "boldFillStyle", "Justificación del espacio académico", style, hRow)
	*index++
	justification, okJustification := data["justificacion"]
	if okJustification && justification != nil {
		mergeSet(template, sheetName, 1, 10, *index, "simpleJustifyStyle", fmt.Sprintf("%v", justification), style, 50)
	} else {
		mergeSet(template, sheetName, 1, 10, *index, "simpleJustifyStyle", "", style, hRow)
	}
	*index++
}

func objectivesSection(template *excelize.File, sheetName string, style map[string]int, data map[string]any, index *int) {
	mergeSet(template, sheetName, 1, 10, *index, "boldFillStyle", "Objetivos del espacio académico", style, hRow)
	*index++

	mergeSet(template, sheetName, 1, 10, *index, "boldLeftLTRStyle", "Objetivo General", style, hRow)
	*index++
	genObjective, okGenObjective := data["objetivo_general"]
	if okGenObjective && genObjective != nil {
		mergeSet(template, sheetName, 1, 10, *index, "simpleJustifyLRStyle", fmt.Sprintf("%v", genObjective), style, 50)
	} else {
		mergeSet(template, sheetName, 1, 10, *index, "simpleJustifyLRStyle", "", style, hRow)
	}
	*index++

	mergeSet(template, sheetName, 1, 10, *index, "boldLeftLRStyle", "Objetivos Específicos", style, hRow)
	*index++
	specObjective, okSpecObjective := data["objetivos_especificos"]
	if okSpecObjective && specObjective != nil {
		specObjectives := ""
		for _, objective := range specObjective.([]any) {
			specObjectives += fmt.Sprintf("•%v \n", objective)
		}
		mergeSet(template, sheetName, 1, 10, *index, "simpleJustifyLRStyle", fmt.Sprintf("%v\n", specObjectives), style, 50)
	} else {
		mergeSet(template, sheetName, 1, 10, *index, "simpleJustifyLRStyle", "", style, hRow)
	}
	*index++
}

func generateLineExcel(template *excelize.File, sheetName string, boldFont string, style map[string]int, message string, index *int) {
	startCell := fmt.Sprintf("A%v", *index)
	endCell := fmt.Sprintf("J%v", *index)
	template.MergeCell(sheetName, startCell, endCell)
	if boldFont == "bold" {
		template.SetCellStyle(sheetName, startCell, endCell, style["boldLeftLRStyle"])
	} else {
		template.SetCellStyle(sheetName, startCell, endCell, style["simpleJustifyLRStyle"])
	}
	template.SetCellValue(sheetName, startCell, message)
}

func purposeSection(template *excelize.File, sheetName string, style map[string]int, data map[string]any, index *int) {
	// --- 1. Título y Encabezados de la Tabla (Versión Corregida y Limpia) ---
	aLabel := fmt.Sprintf("A%v", *index)
	jLabel := fmt.Sprintf("J%v", *index)
	template.MergeCell(sheetName, aLabel, jLabel)
	template.SetCellStyle(sheetName, aLabel, jLabel, style["boldFillStyle"])
	template.SetCellValue(sheetName, aLabel, "Propósitos de Formación y Aprendizaje (PFA)")

	*index++

	propositos, propositosOk := data["propositos"]
	prop, _ := propositos.([]interface{})
	if !propositosOk || propositos == nil || len(prop) == 0 {
		startCell := fmt.Sprintf("A%v", *index)
		endCell := fmt.Sprintf("J%v", *index)
		template.MergeCell(sheetName, startCell, endCell)
		template.SetCellStyle(sheetName, startCell, endCell, style["simpleJustifyLRStyle"])
		template.SetCellValue(sheetName, startCell, "No hay propósitos de formación definidos")
		return
	}

	p, _ := prop[0].(map[string]interface{})

	// Nuevo formato (y clásico): PFA de programa (con campos legacy opcionales)
	if _, hasPFA := p["pfa_programa"]; hasPFA {
		for i, item := range prop {
			itemMap, isMap := item.(map[string]interface{})
			if !isMap {
				continue
			}
			generateLineExcel(template, sheetName, "bold", style, fmt.Sprintf("PROPÓSITO %v", i+1), index)
			*index++
			generateLineExcel(template, sheetName, "bold", style, "PFA del Programa/Proyecto", index)
			*index++
			generateLineExcel(template, sheetName, "simple", style, fmt.Sprintf("%v", itemMap["pfa_programa"]), index)
			*index++
			if value, hasValue := itemMap["pfa_asignatura"]; hasValue && value != nil && fmt.Sprintf("%v", value) != "" {
				generateLineExcel(template, sheetName, "bold", style, "PFA de la Asignatura", index)
				*index++
				generateLineExcel(template, sheetName, "simple", style, fmt.Sprintf("%v", value), index)
				*index++
			}
			if value, hasValue := itemMap["competencias"]; hasValue && value != nil && fmt.Sprintf("%v", value) != "" {
				generateLineExcel(template, sheetName, "bold", style, "Competencias", index)
				*index++
				generateLineExcel(template, sheetName, "simple", style, fmt.Sprintf("%v", value), index)
				*index++
			}
		}
		return
	}

	// Verificar si están los propósitos en versión legacy
	_, exist := p["propositos_formación_legacy"]

	if exist {
		p_data := p["propositos_formación_legacy"].([]interface{})
		for i := 0; i < len(p_data); i += 3 {
			generateLineExcel(template, sheetName, "bold", style, fmt.Sprintf("PROPÓSITO %v", (i+3)/3), index)
			*index++

			generateLineExcel(template, sheetName, "bold", style, "PFA del Programa/Proyecto", index)
			*index++

			generateLineExcel(template, sheetName, "simple", style, p_data[i].(string), index)
			*index++

			generateLineExcel(template, sheetName, "bold", style, "PFA de la Asignatura", index)
			*index++

			generateLineExcel(template, sheetName, "simple", style, p_data[i+1].(string), index)
			*index++

			generateLineExcel(template, sheetName, "bold", style, "Competencias", index)
			*index++

			template.SetRowHeight(sheetName, *index, 50)
			generateLineExcel(template, sheetName, "simple", style, p_data[i+2].(string), index)
			*index++
		}
	} else {
		// se asumen valores en V3 válidos, en su defecto deja valores genericos

		template.SetRowHeight(sheetName, *index, 30)

		// Definimos la información de los encabezados de forma clara.
		// Clave: Columna de inicio. Valor: Título de la columna.
		headerTitles := map[string]string{
			"A": "Competencias",
			"D": "Dominio-Nivel",
			"F": "RA",
			"G": "Resultados de Aprendizaje",
		}

		// Definimos los rangos de las columnas.
		// Clave: Columna de inicio. Valor: Columna de fin.
		colRanges := map[string]string{
			"A": "C",
			"D": "E",
			"F": "F",
			"G": "J",
		}

		// Iteramos de forma ordenada para asegurar que las columnas se creen de izquierda a derecha.
		orderedCols := []string{"A", "D", "F", "G"}
		for _, startCol := range orderedCols {
			title := headerTitles[startCol]
			endCol := colRanges[startCol]
			styleName := "boldStyle" // El estilo es el mismo para todos los encabezados.

			startCell := fmt.Sprintf("%s%d", startCol, *index)
			endCell := fmt.Sprintf("%s%d", endCol, *index)

			template.MergeCell(sheetName, startCell, endCell)
			template.SetCellStyle(sheetName, startCell, endCell, style[styleName])
			template.SetCellValue(sheetName, startCell, title)
		}

		// --- 2. Verificación y Agrupación de Datos ---
		propositosData, ok := data["propositos"]
		if !ok || propositosData == nil {
			*index++
			template.MergeCell(sheetName, fmt.Sprintf("A%v", *index), fmt.Sprintf("J%v", *index))
			template.SetCellStyle(sheetName, fmt.Sprintf("A%v", *index), fmt.Sprintf("J%v", *index), style["simpleStyle"])
			template.SetCellValue(sheetName, fmt.Sprintf("A%v", *index), "No hay propósitos de formación definidos.")
			return
		}

		propositosArray, ok := propositosData.([]any)
		if !ok {
			// El tipo de dato no es una lista como se esperaba.
			return
		}

		// Definimos la estructura para agrupar, igual que en la función del PDF.
		type CompetenciaGroup struct {
			nombre string
			items  []map[string]any
		}
		var competenciasOrdenadas []CompetenciaGroup
		competenciasVistas := make(map[string]bool)

		// Este bucle transforma la lista plana en una lista agrupada por competencia.
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

		// --- 3. Renderizado de Datos Agrupados ---
		// Si no se encontraron competencias para ordenar, no se hace nada más.
		if len(competenciasOrdenadas) == 0 {
			return
		}

		for _, grupo := range competenciasOrdenadas {
			competencia := grupo.nombre
			resultados := grupo.items
			numResultados := len(resultados)

			if numResultados > 0 {
				filaInicialCompetencia := *index + 1

				// Iteramos sobre los resultados de aprendizaje de este grupo.
				for i, resultadoAux := range resultados {
					*index++
					template.SetRowHeight(sheetName, *index, 45) // Asignar una altura de fila.

					// Celda de Competencia (Columnas A-C)
					// El texto de la competencia solo se escribe en la primera fila de su grupo.
					if i == 0 {
						template.SetCellValue(sheetName, fmt.Sprintf("A%v", *index), competencia)
					}
					// El estilo y el merge se aplican a todas las filas del grupo.
					template.MergeCell(sheetName, fmt.Sprintf("A%v", *index), fmt.Sprintf("C%v", *index))
					template.SetCellStyle(sheetName, fmt.Sprintf("A%v", *index), fmt.Sprintf("C%v", *index), style["simpleJustifyStyle"])

					// Celda Dominio-Nivel (Columnas D-E)
					template.MergeCell(sheetName, fmt.Sprintf("D%v", *index), fmt.Sprintf("E%v", *index))
					template.SetCellStyle(sheetName, fmt.Sprintf("D%v", *index), fmt.Sprintf("E%v", *index), style["simpleStyle"])
					template.SetCellValue(sheetName, fmt.Sprintf("D%v", *index), fmt.Sprintf("%v", resultadoAux["dominio"]))

					// Celda RA (Columna F)
					template.SetCellStyle(sheetName, fmt.Sprintf("F%v", *index), fmt.Sprintf("F%v", *index), style["simpleStyle"])
					template.SetCellValue(sheetName, fmt.Sprintf("F%v", *index), fmt.Sprintf("%v", resultadoAux["id"]))

					// Celda Resultados de Aprendizaje (Columnas G-J)
					template.MergeCell(sheetName, fmt.Sprintf("G%v", *index), fmt.Sprintf("J%v", *index))
					template.SetCellStyle(sheetName, fmt.Sprintf("G%v", *index), fmt.Sprintf("J%v", *index), style["simpleJustifyStyle"])
					template.SetCellValue(sheetName, fmt.Sprintf("G%v", *index), fmt.Sprintf("%v", resultadoAux["resultado_detallado"]))
				}

				// Merge vertical de la celda de competencia si abarca más de una fila.
				if numResultados > 1 {
					filaFinalCompetencia := *index
					template.MergeCell(sheetName,
						fmt.Sprintf("A%v", filaInicialCompetencia),
						fmt.Sprintf("C%v", filaFinalCompetencia))
				}
			}
		}
	}
}

func thematicContentSection(template *excelize.File, sheetName string, style map[string]int, data map[string]any, index *int) {
	// CONTENIDOS TEMÁTICOS

	*index++
	aLabel := fmt.Sprintf("A%v", *index)
	jLabel := fmt.Sprintf("J%v", *index)
	template.MergeCell(sheetName, aLabel, jLabel)
	template.SetCellStyle(sheetName, aLabel, jLabel, style["boldFillStyle"])
	template.SetCellValue(sheetName, aLabel, "Contenidos temáticos")

	*index++
	aLabel = fmt.Sprintf("A%v", *index)
	jLabel = fmt.Sprintf("J%v", *index)
	template.MergeCell(sheetName, aLabel, jLabel)
	template.SetCellStyle(sheetName, aLabel, jLabel, style["boldLeftLTRStyle"])
	template.SetCellValue(sheetName, aLabel, "Descripción:")

	*index++
	aLabel = fmt.Sprintf("A%v", *index)
	jLabel = fmt.Sprintf("J%v", *index)
	template.MergeCell(sheetName, aLabel, jLabel)
	template.SetCellStyle(sheetName, aLabel, jLabel, style["simpleJustifyLRStyle"])
	thematicDesc, okThematicDesc := data["contenido_tematico_descripcion"]
	if okThematicDesc && thematicDesc != nil {
		template.SetRowHeight(sheetName, *index, 50)
		template.SetCellValue(sheetName, aLabel, fmt.Sprintf("%v\n\n", thematicDesc))
	} else {
		template.SetCellValue(sheetName, aLabel, "")
	}

	thematicDetails, okThematicDet := data["contenido_tematico_detalle"]
	thematicDetList, _ := thematicDetails.([]any)
	if okThematicDet && thematicDetails != nil && len(thematicDetList) > 0 {
		*index++
		aLabel = fmt.Sprintf("A%v", *index)
		jLabel = fmt.Sprintf("J%v", *index)
		template.MergeCell(sheetName, aLabel, jLabel)
		template.SetCellStyle(sheetName, aLabel, jLabel, style["boldLeftLRStyle"])
		template.SetCellValue(sheetName, aLabel, "Temas y subtemas:")

		*index++
		aLabel = fmt.Sprintf("A%v", *index)
		jLabel = fmt.Sprintf("J%v", *index)
		template.MergeCell(sheetName, aLabel, jLabel)
		template.SetCellStyle(sheetName, aLabel, jLabel, style["simpleJustifyLRStyle"])
		thematicDetStr := ""
		for _, topic := range thematicDetList {
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
		template.SetRowHeight(sheetName, *index, 70)
		template.SetCellValue(sheetName, aLabel, fmt.Sprintf("%v\n\n", thematicDetStr))
	}
	*index++
}

func evaluationSection(template *excelize.File, sheetName string, style map[string]int, data map[string]any, index *int) {
	mergeSet(template, sheetName, 1, 10, *index, "boldFillStyle", "Evaluación", style, hRow)
	*index++

	evaluacionDet, okDet := data["evaluacion_detalle"]
	if !okDet || evaluacionDet == nil {
		mergeSet(template, sheetName, 1, 10, *index, "simpleStyle", "No hay detalles de evaluación definidos", style, hRow)
		*index++
		return
	}

	mergeSet(template, sheetName, 1, 10, *index, "boldLeftStyle", "Descripción:", style, hRow)
	*index++
	mergeSet(template, sheetName, 1, 10, *index, "simpleJustifyStyle", getString(data, "evaluacion_descripcion"), style, 50)
	*index++

	mergeSet(template, sheetName, 1, 10, *index, "boldLeftStyle", "Evaluaciones:", style, hRow)
	*index++

	texto := ""
	if evals, ok := evaluacionDet.([]interface{}); ok {
		for _, item := range evals {
			if value, ok := item.(map[string]interface{}); ok {
				texto += fmt.Sprintf(" - Nombre: %v\n", value["nombre"])
				texto += fmt.Sprintf("\t· Porcentaje: %v\n", value["porcentaje"])
				// Campos legacy (solo si existen)
				if momento, hasMomento := value["momento"]; hasMomento && momento != nil {
					texto += fmt.Sprintf("\t· Momento: %v\n", momento)
				}
				if estrategia, hasEstrategia := value["estrategia"]; hasEstrategia && estrategia != nil {
					texto += fmt.Sprintf("\t· Estrategia: %v\n", estrategia)
				}
				texto += "\n"
			}
		}
	}
	mergeSet(template, sheetName, 1, 10, *index, "simpleJustifyStyle", texto, style, 50)
	*index++
}

func resourcesSection(template *excelize.File, sheetName string, style map[string]int, data map[string]any, index *int) {
	mergeSet(template, sheetName, 1, 10, *index, "boldFillStyle", "Medios y recursos educativos", style, hRow)
	*index++
	resources, okResources := data["medios_recursos"]
	if okResources && resources != nil {
		mergeSet(template, sheetName, 1, 10, *index, "simpleJustifyStyle", fmt.Sprintf("%v", resources), style, 50)
	} else {
		mergeSet(template, sheetName, 1, 10, *index, "simpleJustifyStyle", "", style, hRow)
	}
	*index++
}

func practicesSection(template *excelize.File, sheetName string, style map[string]int, data map[string]any, index *int) {
	mergeSet(template, sheetName, 1, 10, *index, "boldFillStyle", "Prácticas académicas", style, hRow)
	*index++
	practices, okPractices := data["practicas_salidas"]
	if okPractices && practices != nil {
		mergeSet(template, sheetName, 1, 10, *index, "simpleJustifyStyle", fmt.Sprintf("%v", practices), style, 50)
	} else {
		mergeSet(template, sheetName, 1, 10, *index, "simpleJustifyStyle", "", style, hRow)
	}
	*index++
}

func bibliographySection(template *excelize.File, sheetName string, style map[string]int, data map[string]any, index *int) {
	mergeSet(template, sheetName, 1, 10, *index, "boldFillStyle", "Fuentes o referentes", style, hRow)
	*index++

	mergeSet(template, sheetName, 1, 10, *index, "boldLeftLRStyle", "Básicas", style, hRow)
	*index++
	writeBiblio(template, sheetName, style, index, data["bibliografia_basica"])

	mergeSet(template, sheetName, 1, 10, *index, "boldLeftLRStyle", "Complementarias", style, hRow)
	*index++
	writeBiblio(template, sheetName, style, index, data["bibliografia_complementaria"])

	mergeSet(template, sheetName, 1, 10, *index, "boldLeftLRStyle", "Bases de Datos", style, hRow)
	*index++
	writeBiblio(template, sheetName, style, index, data["bibliografia_bases"])

	mergeSet(template, sheetName, 1, 10, *index, "boldLeftLRStyle", "Páginas web", style, hRow)
	*index++
	writeBiblio(template, sheetName, style, index, data["bibliografia_paginas"])
}

func writeBiblio(template *excelize.File, sheetName string, style map[string]int, index *int, raw any) {
	if raw != nil {
		lista := ""
		for _, item := range raw.([]interface{}) {
			lista += fmt.Sprintf("- %v\n", item)
		}
		mergeSet(template, sheetName, 1, 10, *index, "simpleJustifyLRStyle", fmt.Sprintf("%v \n\n", lista), style, 50)
	} else {
		mergeSet(template, sheetName, 1, 10, *index, "simpleJustifyLRStyle", "", style, hRow)
	}
	*index++
}

func trackingSection(template *excelize.File, sheetName string, style map[string]int, data map[string]any, index *int) {
	mergeSet(template, sheetName, 1, 10, *index, "boldFillStyle", "Seguimiento", style, hRow)
	*index++

	mergeSet(template, sheetName, 1, 1, *index, "boldLeftStyle", "Elaboró", style, hRow)
	mergeSet(template, sheetName, 2, 5, *index, "simpleStyle", getString(data, "elaboro"), style, hRow)
	mergeSet(template, sheetName, 6, 6, *index, "boldStyle", "Fecha", style, hRow)
	mergeSet(template, sheetName, 7, 10, *index, "simpleStyle", getString(data, "fecha_elaboro"), style, hRow)
	*index++

	mergeSet(template, sheetName, 1, 1, *index, "boldLeftStyle", "Revisó", style, hRow)
	mergeSet(template, sheetName, 2, 5, *index, "simpleStyle", getString(data, "reviso"), style, hRow)
	mergeSet(template, sheetName, 6, 6, *index, "boldStyle", "Fecha", style, hRow)
	mergeSet(template, sheetName, 7, 10, *index, "simpleStyle", getString(data, "fecha_rev_consejo"), style, hRow)
	*index++

	mergeSet(template, sheetName, 1, 1, *index, "boldLeftStyle", "Aprobó", style, hRow)
	mergeSet(template, sheetName, 2, 5, *index, "simpleStyle", getString(data, "aprobo"), style, hRow)
	mergeSet(template, sheetName, 6, 6, *index, "boldStyle", "Fecha", style, hRow)
	mergeSet(template, sheetName, 7, 10, *index, "simpleStyle", getString(data, "fecha_aprob_consejo"), style, hRow)
	*index++

	mergeSet(template, sheetName, 1, 2, *index, "boldLeftStyle", "Número de acta:", style, hRow)
	mergeSet(template, sheetName, 3, 6, *index, "simpleStyle", getString(data, "num_acta"), style, hRow)
	mergeSet(template, sheetName, 7, 8, *index, "boldLeftStyle", "Versión Syllabus:", style, hRow)
	mergeSet(template, sheetName, 9, 10, *index, "simpleStyle", getString(data, "version_syllabus"), style, hRow)
	*index++
}

func createTemplate(data map[string]any) (excelize.File, error) {
	sheetName := "Sheet1"
	template := excelize.NewFile()
	defer func() {
		if err := template.Close(); err != nil {
			fmt.Println(err)
		}
	}()
	templateStyles := templateStyle(*template)
	createHeader(template, sheetName, templateStyles)
	index := 4
	institutionalSection(template, sheetName, templateStyles, data, &index)
	identificationSection(template, sheetName, templateStyles, data, &index)
	suggestionsSection(template, sheetName, templateStyles, data, &index)
	justificationSection(template, sheetName, templateStyles, data, &index)
	objectivesSection(template, sheetName, templateStyles, data, &index)
	purposeSection(template, sheetName, templateStyles, data, &index)
	thematicContentSection(template, sheetName, templateStyles, data, &index)
	evaluationSection(template, sheetName, templateStyles, data, &index)
	resourcesSection(template, sheetName, templateStyles, data, &index)
	practicesSection(template, sheetName, templateStyles, data, &index)
	bibliographySection(template, sheetName, templateStyles, data, &index)
	trackingSection(template, sheetName, templateStyles, data, &index)
	return *template, nil
}

func encodeFile(template excelize.File) (string, error) {
	bufferExcel, err := template.WriteToBuffer()
	if err != nil {
		fmt.Println("Error en Conversion a base64!!!!!!!!!!!!!!!!!")
		return "", err
	}
	encodedFileExcel := base64.StdEncoding.EncodeToString(bufferExcel.Bytes())
	return encodedFileExcel, nil
}

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	var data map[string]any
	jsonErr := json.Unmarshal([]byte(request.Body), &data)
	if jsonErr != nil {
		return events.APIGatewayProxyResponse{
			Body:       "Error getting data.",
			StatusCode: 500,
		}, nil
	}

	template, errTemplate := createTemplate(data)
	var response map[string]any
	if errTemplate != nil {
		response = map[string]any{
			"statusCode": 500,
			"body": map[string]any{
				"Success": false,
				"Status":  500,
				"Message": "Error error generating syllabus spreadsheet!. Detail: Error in createTemplate",
			},
		}
	} else {
		encodedFileExcel, errEncode := encodeFile(template)
		if errEncode != nil {
			response = map[string]any{
				"statusCode": 500,
				"body": map[string]any{
					"Success": false,
					"Status":  500,
					"Message": "Error error generating syllabus spreadsheet!. Detail: Error in encodeFile",
				},
			}
		} else {
			response = map[string]any{
				"statusCode": 200,
				"body": map[string]any{
					"Success": true,
					"Status":  200,
					"Message": "Syllabus spreadsheet OK",
					"Data":    encodedFileExcel,
				},
			}
		}
	}

	jsonStr, err := json.Marshal(response)

	if err != nil {
		return events.APIGatewayProxyResponse{
			Body:       "Error generating json response.",
			StatusCode: 500,
		}, nil
	}

	return events.APIGatewayProxyResponse{
		Body:       string(jsonStr),
		StatusCode: 200,
	}, nil
}

func main() {
	lambda.Start(handler)
}
