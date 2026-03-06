package cliui

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLogoContainsBrandName(t *testing.T) {
	out := Logo(true)
	assert.Contains(t, out, "D A G R Y N")
}

func TestLogoCompactContainsBrandName(t *testing.T) {
	out := LogoCompact(true)
	assert.Contains(t, out, "DAGRYN")
}

func TestLogoNoColorNoANSI(t *testing.T) {
	out := Logo(true)
	assert.NotContains(t, out, "\033[")
}

func TestPrintLogoWritesToWriter(t *testing.T) {
	buf := new(bytes.Buffer)
	PrintLogo(buf, true)
	assert.Contains(t, buf.String(), "D A G R Y N")
}

func TestBannerIncludesVersion(t *testing.T) {
	out := Banner("v1.2.3", true)
	assert.Contains(t, out, "DAGRYN")
	assert.Contains(t, out, "v1.2.3")
	assert.Contains(t, out, "workflow orchestrator")
}
