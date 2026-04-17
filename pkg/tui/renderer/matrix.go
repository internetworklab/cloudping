package renderer

import (
	"errors"
	"fmt"
	"sort"

	pkgtable "github.com/internetworklab/cloudping/pkg/table"
	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
)

type CSSColor string
type ColorMapFn func(val float64) CSSColor

type PingMatrixRenderer struct {
	ColorMap ColorMapFn
}

func (renderer *PingMatrixRenderer) getColorFn() ColorMapFn {
	if fn := renderer.ColorMap; fn != nil {
		return fn
	}

	defaultColorFn := func(val float64) CSSColor {
		const (
			colorGreen  CSSColor = "#4caf50"
			colorYellow CSSColor = "#ff9800"
			colorRed    CSSColor = "#f44336"
			colorGrey   CSSColor = "#9e9e9e"
		)

		switch {
		case val >= 0 && val < 100:
			return colorGreen
		case val >= 100 && val < 250:
			return colorYellow
		case val >= 250:
			return colorRed
		default:
			return colorGrey
		}
	}

	return defaultColorFn
}

func (renderer *PingMatrixRenderer) Render(mat *PingMatrix) pkgtable.TableLike {
	colorFn := renderer.getColorFn()
	colNames := mat.GetColNames()
	rowNames := mat.GetRowNames()

	// Header row: empty top-left cell + column names
	headerRow := make([]pkgtable.StyledCell, 0, len(colNames)+1)
	headerRow = append(headerRow, pkgtable.StyledCell{TextContent: ""})
	for _, colName := range colNames {
		headerRow = append(headerRow, pkgtable.StyledCell{TextContent: colName})
	}

	cells := make([][]pkgtable.StyledCell, 0, len(rowNames)+1)
	cells = append(cells, headerRow)

	// Data rows: row name as first cell, then values
	for _, rowName := range rowNames {
		row := make([]pkgtable.StyledCell, 0, len(colNames)+1)
		row = append(row, pkgtable.StyledCell{TextContent: rowName})
		for _, colName := range colNames {
			val := mat.GetValue(colName, rowName)
			if val != nil {
				color := string(colorFn(*val))
				row = append(row, pkgtable.StyledCell{
					Style:       pkgtable.CSSInlineStyle(fmt.Sprintf("color: %s;", color)),
					TextContent: fmt.Sprintf("%.1f", *val),
				})
			} else {
				row = append(row, pkgtable.StyledCell{
					TextContent: "-",
				})
			}
		}
		cells = append(cells, row)
	}

	return &pkgtable.StyledTable{Cells: cells}
}

type PingMatrix struct {
	orientation PingMatrixOrientation

	// map location to col/row index
	loc2IdxMap map[string]int

	// map destination to row/col index
	dest2IdxMap map[string]int

	sortedColNames []string
	sortedRowNames []string

	dataStore []*float64
}

var ErrEmptyLocs error = errors.New("locations slice is empty")
var ErrEmptyDests error = errors.New("destinations slice is empty")
var ErrInvalidOrientation error = errors.New("invalid orientation")
var ErrDuplicatedColName error = errors.New("duplicated column name")
var ErrDuplicatedRowName error = errors.New("duplicated row name")

type PingMatrixOrientation string

const TGT_SRC PingMatrixOrientation = "tgt_src" // One row per destination (target), with sources as columns
const SRC_TGT PingMatrixOrientation = "src_tgt" // One row per source, with destinations (targets) as columns

func NewPingMatrix(
	locations []string,
	destinations []string,
	orientation PingMatrixOrientation,
) (*PingMatrix, error) {
	if len(locations) == 0 {
		return nil, ErrEmptyLocs
	}
	if len(destinations) == 0 {
		return nil, ErrEmptyDests
	}

	sortedLocs := make([]string, len(locations))
	copy(sortedLocs, locations)
	sort.Strings(sortedLocs)

	sortedDests := make([]string, len(destinations))
	copy(sortedDests, destinations)
	sort.Strings(sortedDests)

	var sortedColNames []string
	var sortedRowNames []string

	loc2IdxMap := make(map[string]int, len(sortedLocs))
	dest2IdxMap := make(map[string]int, len(sortedDests))

	switch orientation {
	case TGT_SRC:
		// rows = destinations, cols = locations (sources)
		if hasConsecutiveDuplicates(sortedLocs) {
			return nil, ErrDuplicatedColName
		}
		if hasConsecutiveDuplicates(sortedDests) {
			return nil, ErrDuplicatedRowName
		}
		sortedColNames = sortedLocs
		sortedRowNames = sortedDests
		for i, loc := range sortedLocs {
			loc2IdxMap[loc] = i
		}
		for i, dest := range sortedDests {
			dest2IdxMap[dest] = i
		}
	case SRC_TGT:
		// rows = locations (sources), cols = destinations
		if hasConsecutiveDuplicates(sortedDests) {
			return nil, ErrDuplicatedColName
		}
		if hasConsecutiveDuplicates(sortedLocs) {
			return nil, ErrDuplicatedRowName
		}
		sortedColNames = sortedDests
		sortedRowNames = sortedLocs
		for i, loc := range sortedLocs {
			loc2IdxMap[loc] = i
		}
		for i, dest := range sortedDests {
			dest2IdxMap[dest] = i
		}
	default:
		return nil, ErrInvalidOrientation
	}

	numRows := len(sortedRowNames)
	numCols := len(sortedColNames)
	dataStore := make([]*float64, numRows*numCols)

	return &PingMatrix{
		orientation:    orientation,
		loc2IdxMap:     loc2IdxMap,
		dest2IdxMap:    dest2IdxMap,
		sortedColNames: sortedColNames,
		sortedRowNames: sortedRowNames,
		dataStore:      dataStore,
	}, nil
}

func (mat *PingMatrix) getIdx(loc, dest string) int {
	numCols := len(mat.sortedColNames)
	switch mat.orientation {
	case TGT_SRC:
		// row = dest index, col = loc index
		row := mat.dest2IdxMap[dest]
		col := mat.loc2IdxMap[loc]
		return row*numCols + col
	case SRC_TGT:
		// row = loc index, col = dest index
		row := mat.loc2IdxMap[loc]
		col := mat.dest2IdxMap[dest]
		return row*numCols + col
	default:
		return -1
	}
}

func (mat *PingMatrix) WriteSample(loc, dest string, val float64) {
	idx := mat.getIdx(loc, dest)
	if idx >= 0 {
		mat.dataStore[idx] = &val
	}
}

func (mat *PingMatrix) GetColNames() []string {
	colNames := make([]string, len(mat.sortedColNames))
	copy(colNames, mat.sortedColNames)
	return colNames
}

func (mat *PingMatrix) GetRowNames() []string {
	rowNames := make([]string, len(mat.sortedRowNames))
	copy(rowNames, mat.sortedRowNames)
	return rowNames
}

func (mat *PingMatrix) GetValue(colName, rowName string) *float64 {
	var loc, dest string
	switch mat.orientation {
	case TGT_SRC:
		loc = colName
		dest = rowName
	case SRC_TGT:
		loc = rowName
		dest = colName
	default:
		return nil
	}
	idx := mat.getIdx(loc, dest)
	if idx < 0 || idx >= len(mat.dataStore) {
		return nil
	}
	return mat.dataStore[idx]
}

func hasConsecutiveDuplicates(sorted []string) bool {
	return pkgutils.CheckSortedStringsDup(sorted)
}
