package word

import "strconv"

type gridOrigin struct {
	id        string
	row, span int
}

type tableGrid struct {
	id          string
	row, column int
	valid       bool
	previous    map[int]gridOrigin
	current     map[int]gridOrigin
}

func resolveCells(blocks []SourceBlock) {
	scopes := make(map[string]*tableGrid, len(blocks))
	for i := range blocks {
		b := &blocks[i]
		grid := scopes[b.ParentID]
		switch b.Kind {
		case elementTable:
			grid = &tableGrid{id: b.ID, row: -1, valid: true, current: map[int]gridOrigin{}}
		case "tr":
			if grid != nil {
				grid.row++
				grid.previous, grid.current = grid.current, map[int]gridOrigin{}
				grid.column, grid.valid = gridInteger(b.Properties, "trPr", "gridBefore", 0)
			}
		case "tc":
			if grid != nil {
				b.Cell = cellEvidence(*b, grid)
				if !b.Cell.Resolved {
					b.ReviewReasons = appendReasons(b.ReviewReasons, "TABLE_GRID_REQUIRES_REVIEW")
				}
			} else {
				b.ReviewReasons = appendReasons(b.ReviewReasons, "TABLE_GRID_REQUIRES_REVIEW")
			}
		}
		scopes[b.ID] = grid
	}
}

func gridInteger(props []Property, parent, name string, fallback int) (int, bool) {
	if duplicateNamedProperty(props, parent) || duplicateNamedProperty(prop(props, parent).Children, name) {
		return fallback, false
	}
	p := prop(prop(props, parent).Children, name)
	if p.Name == "" {
		return fallback, true
	}
	n, err := strconv.Atoi(attr(p, propertyValue))
	if err != nil || n < fallback || n > 128 {
		return fallback, false
	}
	return n, true
}

func cellEvidence(b SourceBlock, grid *tableGrid) *CellEvidence {
	span, valid := gridInteger(b.Properties, "tcPr", "gridSpan", 1)
	cell := &CellEvidence{TableID: grid.id, Row: grid.row, Column: grid.column, ColumnSpan: span, Resolved: valid && grid.valid && grid.row >= 0 && grid.column+span <= 128}
	properties := prop(b.Properties, "tcPr").Children
	if duplicateProperties(properties) {
		cell.Resolved = false
	}
	if prop(properties, "hMerge").Name != "" {
		cell.Resolved = false
	}
	if merge := prop(properties, "vMerge"); merge.Name != "" {
		cell.VerticalMerge = attr(merge, propertyValue)
		if cell.VerticalMerge == "" {
			cell.VerticalMerge = "continue"
		}
		resolveVerticalMerge(cell, b.ID, grid)
	}
	if !cell.Resolved {
		grid.valid = false
	}
	grid.column += span
	return cell
}

func resolveVerticalMerge(cell *CellEvidence, id string, grid *tableGrid) {
	switch cell.VerticalMerge {
	case "restart":
		cell.MergeOriginID = id
	case "continue":
		origin, found := grid.previous[cell.Column]
		if !found || origin.row != cell.Row-1 || origin.span != cell.ColumnSpan {
			cell.Resolved = false
		} else {
			cell.MergeOriginID = origin.id
		}
	default:
		cell.Resolved = false
	}
	if cell.Resolved && cell.MergeOriginID != "" {
		grid.current[cell.Column] = gridOrigin{id: cell.MergeOriginID, row: cell.Row, span: cell.ColumnSpan}
	}
}
