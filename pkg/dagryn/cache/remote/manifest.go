package remote

import "encoding/json"

// Manifest maps relative file paths to their content digests.
// It is stored as JSON at the action key.
type Manifest struct {
	Files map[string]*Digest `json:"files"`
}

// MarshalManifest serializes a Manifest to JSON.
func MarshalManifest(m *Manifest) ([]byte, error) {
	return json.Marshal(m)
}

// UnmarshalManifest deserializes a Manifest from JSON.
func UnmarshalManifest(data []byte) (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}
