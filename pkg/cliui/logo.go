package cliui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Logo lines — a hexagon matching the Dagryn favicon.
// The raw shape is stored plain; color is applied at render time.
var logoLines = []string{
	`        _______________        `,
	`       /               \       `,
	`      /                 \      `,
	`     /                   \     `,
	`    /                     \    `,
	`   /      D A G R Y N      \   `,
	`   \                       /   `,
	`    \                     /    `,
	`     \                   /     `,
	`      \                 /      `,
	`       \_______________ /       `,
}

// compact logo for narrow terminals / inline use.
var logoCompact = []string{
	`   ___________   `,
	`  /           \  `,
	` /   DAGRYN    \ `,
	` \             / `,
	`  \___________/  `,
}

// Brand styles.
var (
	styleLogo     = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true) // bright white
	styleLogoName = lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Bold(true) // cyan
)

// Logo returns the rendered multi-line Dagryn logo string.
// If noColor is true no ANSI codes are emitted.
func Logo(noColor bool) string {
	return renderLogo(logoLines, noColor)
}

// LogoCompact returns a smaller version of the logo.
func LogoCompact(noColor bool) string {
	return renderLogo(logoCompact, noColor)
}

func renderLogo(lines []string, noColor bool) string {
	var b strings.Builder
	for _, line := range lines {
		if noColor {
			b.WriteString(line)
		} else {
			// Highlight the text inside the hexagon
			if strings.Contains(line, "DAGRYN") || strings.Contains(line, "D A G R Y N") {
				// Split around the name and color it differently
				rendered := colorLogoLine(line)
				b.WriteString(rendered)
			} else {
				b.WriteString(styleLogo.Render(line))
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// colorLogoLine renders a logo line, highlighting the brand name in cyan.
func colorLogoLine(line string) string {
	for _, name := range []string{"D A G R Y N", "DAGRYN"} {
		if before, after, found := strings.Cut(line, name); found {
			return styleLogo.Render(before) +
				styleLogoName.Render(name) +
				styleLogo.Render(after)
		}
	}
	return styleLogo.Render(line)
}

// PrintLogo writes the full logo to w.
func PrintLogo(w io.Writer, noColor bool) {
	_, _ = fmt.Fprint(w, Logo(noColor))
}

// Banner returns the logo plus a tagline, suitable for the root help header.
func Banner(version string, noColor bool) string {
	var b strings.Builder
	b.WriteString(LogoCompact(noColor))
	tag := fmt.Sprintf("  Local-first workflow orchestrator %s", version)
	if !noColor {
		tag = StyleDim.Render(tag)
	}
	b.WriteString(tag)
	b.WriteByte('\n')
	return b.String()
}
