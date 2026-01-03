package ui

import (
	"os"

	"github.com/olekukonko/tablewriter"
)

// Table is a wrapper around tablewriter for consistent table formatting.
type Table struct {
	writer *tablewriter.Table
}

// NewTable creates a new table with headers.
func NewTable(headers []string) *Table {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(headers)
	table.SetBorder(false)
	table.SetHeaderLine(false)
	table.SetColumnSeparator("  ")
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetAutoWrapText(false)

	// Style the header
	table.SetHeaderColor(
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
	)

	return &Table{writer: table}
}

// NewTableNoHeader creates a new table without headers.
func NewTableNoHeader() *Table {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetBorder(false)
	table.SetColumnSeparator("  ")
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetAutoWrapText(false)

	return &Table{writer: table}
}

// AddRow adds a row to the table.
func (t *Table) AddRow(row []string) {
	t.writer.Append(row)
}

// AddColoredRow adds a row with custom colors.
func (t *Table) AddColoredRow(row []string, colors []tablewriter.Colors) {
	t.writer.Rich(row, colors)
}

// Render prints the table.
func (t *Table) Render() {
	t.writer.Render()
}

// SetColumnWidths sets minimum column widths.
func (t *Table) SetColumnWidths(widths []int) {
	for i, w := range widths {
		t.writer.SetColMinWidth(i, w)
	}
}

// TableColor provides color constants for table cells.
var TableColor = struct {
	Green  tablewriter.Colors
	Yellow tablewriter.Colors
	Red    tablewriter.Colors
	Blue   tablewriter.Colors
	Cyan   tablewriter.Colors
	Normal tablewriter.Colors
}{
	Green:  tablewriter.Colors{tablewriter.FgGreenColor},
	Yellow: tablewriter.Colors{tablewriter.FgYellowColor},
	Red:    tablewriter.Colors{tablewriter.FgRedColor},
	Blue:   tablewriter.Colors{tablewriter.FgBlueColor},
	Cyan:   tablewriter.Colors{tablewriter.FgCyanColor},
	Normal: tablewriter.Colors{},
}

