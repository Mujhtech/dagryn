package cliui

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTableOutput(t *testing.T) {
	buf := new(bytes.Buffer)
	tbl := NewTableNoColor(buf, "NAME", "STATUS", "SIZE")
	tbl.Row("build", "ok", "1.2MB")
	tbl.Row("test", "fail", "3.4MB")
	tbl.Flush()

	out := buf.String()
	assert.Contains(t, out, "NAME")
	assert.Contains(t, out, "build")
	assert.Contains(t, out, "test")
	assert.Contains(t, out, "fail")
}

func TestTableNoHeaders(t *testing.T) {
	buf := new(bytes.Buffer)
	tbl := NewTableNoColor(buf)
	tbl.Row("a", "b")
	tbl.Flush()

	out := buf.String()
	assert.Contains(t, out, "a")
	assert.Contains(t, out, "b")
}
