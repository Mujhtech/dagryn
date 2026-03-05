package scim

import "time"

// SCIM 2.0 resource types per RFC 7643/7644.

// SCIMUser represents a SCIM 2.0 User resource.
type SCIMUser struct {
	Schemas    []string  `json:"schemas"`
	ID         string    `json:"id"`
	ExternalID string    `json:"externalId,omitempty"`
	UserName   string    `json:"userName"`
	Name       *SCIMName `json:"name,omitempty"`
	Emails     []SCIMEmail `json:"emails,omitempty"`
	Active     bool      `json:"active"`
	DisplayName string   `json:"displayName,omitempty"`
	Meta       SCIMMeta  `json:"meta"`
}

// SCIMName represents a SCIM name.
type SCIMName struct {
	GivenName  string `json:"givenName,omitempty"`
	FamilyName string `json:"familyName,omitempty"`
	Formatted  string `json:"formatted,omitempty"`
}

// SCIMEmail represents a SCIM email.
type SCIMEmail struct {
	Value   string `json:"value"`
	Type    string `json:"type,omitempty"`
	Primary bool   `json:"primary,omitempty"`
}

// SCIMGroup represents a SCIM 2.0 Group resource.
type SCIMGroup struct {
	Schemas     []string       `json:"schemas"`
	ID          string         `json:"id"`
	DisplayName string         `json:"displayName"`
	Members     []SCIMMember   `json:"members,omitempty"`
	Meta        SCIMMeta       `json:"meta"`
}

// SCIMMember represents a SCIM group member reference.
type SCIMMember struct {
	Value   string `json:"value"`
	Display string `json:"display,omitempty"`
	Ref     string `json:"$ref,omitempty"`
}

// SCIMMeta represents SCIM resource metadata.
type SCIMMeta struct {
	ResourceType string    `json:"resourceType"`
	Created      time.Time `json:"created"`
	LastModified time.Time `json:"lastModified"`
	Location     string    `json:"location,omitempty"`
}

// ListResponse represents a SCIM 2.0 list response.
type ListResponse struct {
	Schemas      []string    `json:"schemas"`
	TotalResults int         `json:"totalResults"`
	StartIndex   int         `json:"startIndex"`
	ItemsPerPage int         `json:"itemsPerPage"`
	Resources    interface{} `json:"Resources"`
}

// PatchOp represents a SCIM 2.0 PATCH operation.
type PatchOp struct {
	Schemas    []string         `json:"schemas"`
	Operations []PatchOperation `json:"Operations"`
}

// PatchOperation represents a single SCIM PATCH operation.
type PatchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path,omitempty"`
	Value interface{} `json:"value,omitempty"`
}

// SCIMError represents a SCIM 2.0 error response.
type SCIMError struct {
	Schemas  []string `json:"schemas"`
	Detail   string   `json:"detail"`
	Status   int      `json:"status"`
	ScimType string   `json:"scimType,omitempty"`
}

const (
	// SCIM schemas
	SchemaUser          = "urn:ietf:params:scim:schemas:core:2.0:User"
	SchemaGroup         = "urn:ietf:params:scim:schemas:core:2.0:Group"
	SchemaListResponse  = "urn:ietf:params:scim:api:messages:2.0:ListResponse"
	SchemaPatchOp       = "urn:ietf:params:scim:api:messages:2.0:PatchOp"
	SchemaError         = "urn:ietf:params:scim:api:messages:2.0:Error"
)
