package content

import "strings"

type grid struct {
	rows [][MaxColumns]bool
}

func (v *validator) table(value any) (string, bool) {
	obj, ok := object(value, typeKey, "rows")
	rows, valid := array(obj["rows"], 1, MaxRows)
	if !ok || !valid {
		return "", false
	}
	occupied := grid{rows: make([][MaxColumns]bool, len(rows))}
	parts := make([]string, 0, len(rows))
	for row, cells := range rows {
		part, valid := v.row(cells, row, &occupied)
		if !valid {
			return "", false
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, "\n"), occupied.rectangular()
}

func (v *validator) row(value any, row int, occupied *grid) (string, bool) {
	cells, valid := array(value, 0, MaxColumns)
	if !valid {
		return "", false
	}
	parts := make([]string, 0, len(cells))
	column := 0
	for _, cell := range cells {
		for column < MaxColumns && occupied.rows[row][column] {
			column++
		}
		part, width, valid := v.cell(cell, row, column, occupied)
		if !valid {
			return "", false
		}
		column += width
		parts = append(parts, part)
	}
	return strings.Join(parts, "\t"), true
}

func (v *validator) cell(value any, row, column int, occupied *grid) (string, int, bool) {
	v.nodes++
	obj, ok := object(value, "header", "rowSpan", "colSpan", contentKey)
	_, header := obj["header"].(bool)
	height, validHeight := positiveInteger(obj["rowSpan"], MaxRows)
	width, validWidth := positiveInteger(obj["colSpan"], MaxColumns)
	if !ok || !header || !validHeight || !validWidth || !occupied.fill(row, column, height, width) {
		return "", 0, false
	}
	part, valid := v.blocks(obj[contentKey], true, "\n")
	return part, width, valid
}

func (g *grid) fill(row, column, height, width int) bool {
	if row+height > len(g.rows) || column+width > MaxColumns {
		return false
	}
	for y := row; y < row+height; y++ {
		for x := column; x < column+width; x++ {
			if g.rows[y][x] {
				return false
			}
			g.rows[y][x] = true
		}
	}
	return true
}

func (g *grid) rectangular() bool {
	width := 0
	for column := range MaxColumns {
		if g.rows[0][column] {
			width++
		}
	}
	for _, row := range g.rows {
		for column, filled := range row {
			if filled != (column < width) {
				return false
			}
		}
	}
	return width > 0
}
