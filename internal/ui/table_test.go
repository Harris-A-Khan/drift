package ui

import (
	"testing"

	"github.com/olekukonko/tablewriter"
)

func TestNewTable(t *testing.T) {
	headers := []string{"Name", "Value", "Status"}
	table := NewTable(headers)
	if table == nil {
		t.Error("NewTable returned nil")
	}
	if table.writer == nil {
		t.Error("NewTable did not initialize writer")
	}
}

func TestNewTableNoHeader(t *testing.T) {
	table := NewTableNoHeader()
	if table == nil {
		t.Error("NewTableNoHeader returned nil")
	}
	if table.writer == nil {
		t.Error("NewTableNoHeader did not initialize writer")
	}
}

func TestTableAddRow(t *testing.T) {
	table := NewTable([]string{"Col1", "Col2"})

	// Should not panic when adding rows
	table.AddRow([]string{"val1", "val2"})
	table.AddRow([]string{"val3", "val4"})

	// If we got here without panicking, the test passes
}

func TestTableSetColumnWidths(t *testing.T) {
	table := NewTable([]string{"Name", "Description"})

	// Should not panic when setting widths
	table.SetColumnWidths([]int{20, 40})
	table.AddRow([]string{"short", "longer text here"})

	// If we got here without panicking, the test passes
}

func TestTableColors(t *testing.T) {
	// Test that TableColor struct has expected colors
	if len(TableColor.Green) == 0 {
		t.Error("TableColor.Green should not be empty")
	}
	if len(TableColor.Yellow) == 0 {
		t.Error("TableColor.Yellow should not be empty")
	}
	if len(TableColor.Red) == 0 {
		t.Error("TableColor.Red should not be empty")
	}
	if len(TableColor.Blue) == 0 {
		t.Error("TableColor.Blue should not be empty")
	}
	if len(TableColor.Cyan) == 0 {
		t.Error("TableColor.Cyan should not be empty")
	}
	// Normal should be empty (no color)
	if len(TableColor.Normal) != 0 {
		t.Error("TableColor.Normal should be empty")
	}
}

func TestTableAddColoredRow(t *testing.T) {
	table := NewTable([]string{"Name", "Status"})

	// Should not panic when adding colored rows
	table.AddColoredRow(
		[]string{"test", "ok"},
		[]tablewriter.Colors{TableColor.Normal, TableColor.Green},
	)

	// If we got here without panicking, the test passes
}
