package table

import (
	"fmt"
	"strings"
)

type Row struct {
	Cells []string
}

type Table struct {
	Rows []Row
}

func (tb *Table) GetHumanReadableText(colGap int, rowGap int, maxColWidth int) string {
	if len(tb.Rows) == 0 {
		return "(No data)"
	}

	// Calculate max width for each column
	numCols := 0
	for _, row := range tb.Rows {
		if len(row.Cells) > numCols {
			numCols = len(row.Cells)
		}
	}
	if numCols == 0 {
		return "(No data)"
	}
	colWidths := make([]int, numCols)
	for _, row := range tb.Rows {
		for colIdx, cell := range row.Cells {
			cellLen := len(cell)
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
	writeRow := func(row Row) {
		for colIdx := 0; colIdx < numCols; colIdx++ {
			cell := ""
			if colIdx < len(row.Cells) {
				cell = truncateCell(row.Cells[colIdx])
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
	for rowIdx, row := range tb.Rows {
		// Add row gap between hops (blank rows)
		if rowIdx > 0 && len(row.Cells) == 0 {
			sb.WriteString("\n")
			continue
		}

		writeRow(row)
	}

	return sb.String()
}

func (tb *Table) GetReadableHTMLTable() string {
	if len(tb.Rows) == 0 {
		return "<table></table>"
	}

	var sb strings.Builder
	sb.WriteString("<table border=\"1\" style=\"border-collapse: collapse;\">")

	for rowIdx, row := range tb.Rows {
		// Skip empty separator rows
		if len(row.Cells) == 0 {
			continue
		}

		sb.WriteString("<tr>")
		for _, cell := range row.Cells {
			if rowIdx == 0 {
				fmt.Fprintf(&sb, "<th style=\"border:1px solid;text-align:left\">%s</th>", tb.escapeHTML(cell))
			} else {
				fmt.Fprintf(&sb, "<td style=\"border:1px solid;text-align:left\">%s</td>", tb.escapeHTML(cell))
			}
		}
		sb.WriteString("</tr>")
	}

	sb.WriteString("</table>")
	return sb.String()
}

func (tb *Table) escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}
