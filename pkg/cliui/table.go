package cliui

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

// Table is a convenience wrapper around tabwriter for consistent CLI tables.
type Table struct {
	tw      *tabwriter.Writer
	noColor bool
	headers []string
}

// NewTable creates a table that writes to w.
func NewTable(w io.Writer, headers ...string) *Table {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	t := &Table{
		tw:      tw,
		noColor: false,
		headers: headers,
	}
	if len(headers) > 0 {
		t.writeHeader()
	}
	return t
}

// NewTableNoColor creates a table with color disabled.
func NewTableNoColor(w io.Writer, headers ...string) *Table {
	t := NewTable(w, headers...)
	t.noColor = true
	return t
}

func (t *Table) writeHeader() {
	row := strings.Join(t.headers, "\t")
	if !t.noColor {
		row = StyleDim.Render(row)
	}
	_, _ = fmt.Fprintln(t.tw, row)
}

// Row appends a data row.
func (t *Table) Row(cols ...string) {
	_, _ = fmt.Fprintln(t.tw, strings.Join(cols, "\t"))
}

// Flush writes any buffered data to the underlying writer.
func (t *Table) Flush() {
	_ = t.tw.Flush()
}
