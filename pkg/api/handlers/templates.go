package handlers

import (
	"net/http"

	"github.com/mujhtech/dagryn/pkg/http/response"
	"github.com/mujhtech/dagryn/pkg/templates"
)

// SampleTemplateResponse is the response for the sample template endpoint.
type SampleTemplateResponse struct {
	Language    string `json:"language" example:"Go"`
	ProjectType string `json:"project_type" example:"go"`
	Template    string `json:"template"`
}

// GetSampleTemplate godoc
//
//	@Summary		Get a sample dagryn.toml template
//	@Description	Returns a sample dagryn.toml template based on the repository language
//	@Tags			templates
//	@Produce		json
//	@Param			language	query		string	true	"Repository language (e.g. Go, Python, JavaScript)"
//	@Success		200			{object}	SampleTemplateResponse
//	@Failure		400			{object}	response.Response
//	@Router			/api/v1/templates/sample [get]
func (h *Handler) GetSampleTemplate(w http.ResponseWriter, r *http.Request) {
	language := r.URL.Query().Get("language")
	if language == "" {
		_ = response.BadRequest(w, r, nil)
		return
	}

	projectType := templates.ProjectTypeFromGitHubLanguage(language)
	tmpl := templates.GetTemplate(projectType, "")

	_ = response.Ok(w, r, "Sample template retrieved", SampleTemplateResponse{
		Language:    language,
		ProjectType: string(projectType),
		Template:    tmpl,
	})
}
