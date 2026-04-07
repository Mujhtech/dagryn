package github

import "time"

// Repository represents a GitHub repository.
type Repository struct {
	ID            int64  `json:"id"`
	FullName      string `json:"full_name"`
	CloneURL      string `json:"clone_url"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
	Language      string `json:"language"`
}

// Commit represents a GitHub commit.
type Commit struct {
	SHA    string `json:"sha"`
	Commit struct {
		Message string `json:"message"`
		Author  struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		} `json:"author"`
	} `json:"commit"`
	Author struct {
		AvatarURL string `json:"avatar_url"`
	} `json:"author"`
}

// PullRequest represents a minimal GitHub pull request reference.
type PullRequest struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
}

// PRFile represents a file in a pull request diff.
type PRFile struct {
	Filename string `json:"filename"`
	Patch    string `json:"patch"`
}

// ContentItem represents a file/directory listing entry from the Contents API.
type ContentItem struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"`
}

// ContentFile represents a single file response from the Contents API.
type ContentFile struct {
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
}

// CheckRunOutput represents the output section of a GitHub check run.
type CheckRunOutput struct {
	Title   string `json:"title,omitempty"`
	Summary string `json:"summary,omitempty"`
	Text    string `json:"text,omitempty"`
}

// CheckRunRequest is used to create or update a GitHub check run.
type CheckRunRequest struct {
	Name        string          `json:"name,omitempty"`
	HeadSHA     string          `json:"head_sha,omitempty"`
	Status      string          `json:"status,omitempty"`       // queued, in_progress, completed
	Conclusion  string          `json:"conclusion,omitempty"`   // success, failure, cancelled, neutral, timed_out, action_required, stale, skipped
	DetailsURL  string          `json:"details_url,omitempty"`  // link back to Dagryn
	StartedAt   *time.Time      `json:"started_at,omitempty"`   // RFC3339
	CompletedAt *time.Time      `json:"completed_at,omitempty"` // RFC3339
	Output      *CheckRunOutput `json:"output,omitempty"`
}

// CommitStatusRequest represents a request to set a commit status.
type CommitStatusRequest struct {
	State       string `json:"state"`
	Description string `json:"description"`
	Context     string `json:"context"`
	TargetURL   string `json:"target_url,omitempty"`
}

// GitRef represents a Git reference response.
type GitRef struct {
	Object struct {
		SHA string `json:"sha"`
	} `json:"object"`
}

// CreateFileRequest represents a request to create or update a file via the Contents API.
type CreateFileRequest struct {
	Message string `json:"message"`
	Content string `json:"content"` // base64-encoded
	Branch  string `json:"branch"`
}

// CreatePRRequest represents a request to create a pull request.
type CreatePRRequest struct {
	Title string `json:"title"`
	Head  string `json:"head"`
	Base  string `json:"base"`
	Body  string `json:"body"`
}

// InstallationReposResponse is the response from listing installation repositories.
type InstallationReposResponse struct {
	Repositories []Repository `json:"repositories"`
}
