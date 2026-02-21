package initcmd

import "github.com/mujhtech/dagryn/pkg/templates"

// GetTemplate returns the dagryn.toml template for a given project type and package manager.
// Delegates to the shared templates package.
func GetTemplate(projectType ProjectType, pm PackageManager) string {
	return templates.GetTemplate(projectType, pm)
}

// GetTemplateInfo returns template name and description for listing
type TemplateInfo = templates.TemplateInfo

// GetAllTemplateInfos returns info about all available templates.
// Delegates to the shared templates package.
func GetAllTemplateInfos() []TemplateInfo {
	return templates.GetAllTemplateInfos()
}
