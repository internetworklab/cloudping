package table

import (
	"fmt"
	"strings"
)

type TableLike interface {
	GetHumanReadableText(colGap int, rowGap int, maxColWidth int) string
	GetReadableHTMLTable() string
}

type CSSInlineStyle string
type StyledCell struct {
	Style       CSSInlineStyle
	TextContent string
}

type StyledTable struct {
	Cells [][]StyledCell
}

func (tb *StyledTable) GetHumanReadableText(colGap int, rowGap int, maxColWidth int) string {
	if len(tb.Cells) == 0 {
		return "(No data)"
	}

	// Calculate max width for each column
	numCols := 0
	for _, row := range tb.Cells {
		if len(row) > numCols {
			numCols = len(row)
		}
	}
	if numCols == 0 {
		return "(No data)"
	}
	colWidths := make([]int, numCols)
	for _, row := range tb.Cells {
		for colIdx, cell := range row {
			cellLen := len(cell.TextContent)
			if maxColWidth > 0 && cellLen > maxColWidth {
				cellLen = maxColWidth
			}
			if colIdx < numCols && cellLen > colWidths[colIdx] {
				colWidths[colIdx] = cellLen
			}
		}
	}

	var sb strings.Builder

	// Helper function to truncate a cell if it exceeds maxColWidth
	truncateCell := func(cell string) string {
		if maxColWidth > 0 && len(cell) > maxColWidth {
			if maxColWidth <= 3 {
				return cell[:maxColWidth]
			}
			return cell[:maxColWidth-3] + "..."
		}
		return cell
	}

	// Helper function to write a row with aligned columns
	writeRow := func(row []StyledCell) {
		for colIdx := range numCols {
			cell := ""
			if colIdx < len(row) {
				cell = truncateCell(row[colIdx].TextContent)
			}
			// Pad cell to max width for this column (left-aligned)
			fmt.Fprintf(&sb, "%-*s", colWidths[colIdx], cell)
			if colIdx < numCols-1 && colGap > 0 {
				sb.WriteString(strings.Repeat(" ", colGap))
			}
		}
		sb.WriteString("\n")
	}

	// Write all rows
	for rowIdx, row := range tb.Cells {
		// Add row gap between hops (blank rows)
		if rowIdx > 0 && len(row) == 0 {
			sb.WriteString("\n")
			continue
		}

		writeRow(row)
	}

	return sb.String()
}

func (tb *StyledTable) GetReadableHTMLTable() string {
	if len(tb.Cells) == 0 {
		return "<table></table>"
	}

	var sb strings.Builder
	sb.WriteString("<table border=\"1\" style=\"border-collapse: collapse;\">")

	for rowIdx, row := range tb.Cells {
		if len(row) == 0 {
			continue
		}

		sb.WriteString("<tr>")
		for _, cell := range row {
			escapedContent := escapeHTML(cell.TextContent)
			tag := "td"
			if rowIdx == 0 {
				tag = "th"
			}
			if cell.Style != "" {
				fmt.Fprintf(&sb, "<%s style=\"border:1px solid;text-align:left;%s\">%s</%s>", tag, string(cell.Style), escapedContent, tag)
			} else {
				fmt.Fprintf(&sb, "<%s style=\"border:1px solid;text-align:left\">%s</%s>", tag, escapedContent, tag)
			}
		}
		sb.WriteString("</tr>")
	}

	sb.WriteString("</table>")
	return sb.String()
}

type Row struct {
	Cells []string
}

type Table struct {
	Rows []Row
}

func (tb *Table) GetHumanReadableText(colGap int, rowGap int, maxColWidth int) string {
	cells := make([][]StyledCell, len(tb.Rows))
	for i, row := range tb.Rows {
		styledRow := make([]StyledCell, len(row.Cells))
		for j, cell := range row.Cells {
			styledRow[j] = StyledCell{TextContent: cell}
		}
		cells[i] = styledRow
	}
	return (&StyledTable{Cells: cells}).GetHumanReadableText(colGap, rowGap, maxColWidth)
}

func (tb *Table) GetReadableHTMLTable() string {
	cells := make([][]StyledCell, len(tb.Rows))
	for i, row := range tb.Rows {
		styledRow := make([]StyledCell, len(row.Cells))
		for j, cell := range row.Cells {
			styledRow[j] = StyledCell{TextContent: cell}
		}
		cells[i] = styledRow
	}
	return (&StyledTable{Cells: cells}).GetReadableHTMLTable()
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}
