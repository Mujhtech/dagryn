package sso

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/crewjam/saml"
	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/database/repo"
)

// Config holds SSO service configuration.
type Config struct {
	SPCertPEM string
	SPKeyPEM  string
	BaseURL   string
}

// Service provides SSO operations.
type Service struct {
	config   Config
	ssoRepo  repo.SSOStore
	userRepo repo.UserStore
	teamRepo repo.TeamStore
}

// NewService creates a new SSO service.
func NewService(config Config, ssoRepo repo.SSOStore, userRepo repo.UserStore, teamRepo repo.TeamStore) *Service {
	return &Service{
		config:   config,
		ssoRepo:  ssoRepo,
		userRepo: userRepo,
		teamRepo: teamRepo,
	}
}

// BuildSP constructs a SAML ServiceProvider from a connection config.
func (s *Service) BuildSP(conn *models.SSOConnection) (*saml.ServiceProvider, error) {
	keyPair, err := tls.X509KeyPair([]byte(s.config.SPCertPEM), []byte(s.config.SPKeyPEM))
	if err != nil {
		return nil, fmt.Errorf("parse sp key pair: %w", err)
	}

	keyPair.Leaf, err = x509.ParseCertificate(keyPair.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("parse sp certificate: %w", err)
	}

	rootURL, err := url.Parse(conn.SPEntityID)
	if err != nil {
		return nil, fmt.Errorf("parse sp entity id: %w", err)
	}

	acsURL, err := url.Parse(conn.SPAcsURL)
	if err != nil {
		return nil, fmt.Errorf("parse sp acs url: %w", err)
	}

	// Parse IdP metadata
	var idpMetadata *saml.EntityDescriptor
	if conn.IDPMetadataXML != nil && *conn.IDPMetadataXML != "" {
		idpMetadata, err = ParseIDPMetadataXML([]byte(*conn.IDPMetadataXML))
		if err != nil {
			return nil, fmt.Errorf("parse idp metadata: %w", err)
		}
	} else {
		// Build minimal metadata from connection fields
		idpSSOURL, _ := url.Parse(conn.IDPSsoURL)

		// Parse IdP certificate if provided
		var idpCerts []saml.IDPSSODescriptor
		if conn.Certificate != "" {
			block, _ := pem.Decode([]byte(conn.Certificate))
			if block != nil {
				idpCerts = []saml.IDPSSODescriptor{
					{
						SSODescriptor: saml.SSODescriptor{
							RoleDescriptor: saml.RoleDescriptor{
								KeyDescriptors: []saml.KeyDescriptor{
									{
										Use: "signing",
										KeyInfo: saml.KeyInfo{
											X509Data: saml.X509Data{
												X509Certificates: []saml.X509Certificate{
													{Data: string(block.Bytes)},
												},
											},
										},
									},
								},
							},
						},
						SingleSignOnServices: []saml.Endpoint{
							{
								Binding:  saml.HTTPRedirectBinding,
								Location: idpSSOURL.String(),
							},
						},
					},
				}
			}
		}

		idpMetadata = &saml.EntityDescriptor{
			EntityID:          conn.IDPEntityID,
			IDPSSODescriptors: idpCerts,
		}
	}

	sp := &saml.ServiceProvider{
		EntityID:          conn.SPEntityID,
		Key:               keyPair.PrivateKey.(*rsa.PrivateKey),
		Certificate:       keyPair.Leaf,
		AcsURL:            *acsURL,
		MetadataURL:       *rootURL,
		IDPMetadata:       idpMetadata,
		AllowIDPInitiated: true,
	}

	return sp, nil
}

// GenerateMetadata generates SP metadata XML for a team.
func (s *Service) GenerateMetadata(ctx context.Context, teamSlug string) ([]byte, error) {
	team, err := s.teamRepo.GetBySlug(ctx, teamSlug)
	if err != nil {
		return nil, fmt.Errorf("get team: %w", err)
	}

	conn, err := s.ssoRepo.GetConnectionByTeamID(ctx, team.ID)
	if err != nil {
		return nil, fmt.Errorf("get sso connection: %w", err)
	}

	sp, err := s.BuildSP(conn)
	if err != nil {
		return nil, fmt.Errorf("build sp: %w", err)
	}

	metadata := sp.Metadata()
	data, err := xml.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal metadata: %w", err)
	}
	return data, nil
}

// InitiateLogin creates an AuthnRequest and returns the IdP redirect URL.
func (s *Service) InitiateLogin(ctx context.Context, teamSlug, redirectURL string) (string, error) {
	team, err := s.teamRepo.GetBySlug(ctx, teamSlug)
	if err != nil {
		return "", fmt.Errorf("get team: %w", err)
	}

	conn, err := s.ssoRepo.GetConnectionByTeamID(ctx, team.ID)
	if err != nil {
		return "", fmt.Errorf("get sso connection: %w", err)
	}

	sp, err := s.BuildSP(conn)
	if err != nil {
		return "", fmt.Errorf("build sp: %w", err)
	}

	// Generate relay state for CSRF protection
	relayState := uuid.New().String()
	state := &models.SSOState{
		ConnectionID: conn.ID,
		RelayState:   relayState,
		RedirectURL:  redirectURL,
		ExpiresAt:    time.Now().Add(5 * time.Minute),
	}
	if err := s.ssoRepo.CreateState(ctx, state); err != nil {
		return "", fmt.Errorf("create sso state: %w", err)
	}

	// Build AuthnRequest
	authnRequest, err := sp.MakeAuthenticationRequest(
		sp.GetSSOBindingLocation(saml.HTTPRedirectBinding),
		saml.HTTPRedirectBinding,
		saml.HTTPPostBinding,
	)
	if err != nil {
		return "", fmt.Errorf("make authn request: %w", err)
	}

	redirectTo, err := authnRequest.Redirect(relayState, sp)
	if err != nil {
		return "", fmt.Errorf("build redirect url: %w", err)
	}

	return redirectTo.String(), nil
}

// ProcessACS validates the SAML response and returns the authenticated user.
// The http.Request is required because crewjam/saml reads the SAMLResponse from the request form data.
func (s *Service) ProcessACS(ctx context.Context, r *http.Request, teamSlug, relayState string) (*models.User, string, error) {
	team, err := s.teamRepo.GetBySlug(ctx, teamSlug)
	if err != nil {
		return nil, "", fmt.Errorf("get team: %w", err)
	}

	conn, err := s.ssoRepo.GetConnectionByTeamID(ctx, team.ID)
	if err != nil {
		return nil, "", fmt.Errorf("get sso connection: %w", err)
	}

	// Validate relay state
	state, err := s.ssoRepo.GetStateByRelayState(ctx, relayState)
	if err != nil {
		return nil, "", fmt.Errorf("invalid relay state: %w", err)
	}
	// Delete the state to prevent replay
	_ = s.ssoRepo.DeleteState(ctx, state.ID)

	sp, err := s.BuildSP(conn)
	if err != nil {
		return nil, "", fmt.Errorf("build sp: %w", err)
	}

	// Parse and validate the SAML response
	assertion, err := sp.ParseResponse(r, []string{state.RelayState})
	if err != nil {
		return nil, "", fmt.Errorf("parse saml response: %w", err)
	}

	// Extract user attributes
	attrs := ExtractAttributes(assertion)
	if attrs.Email == "" {
		return nil, "", fmt.Errorf("no email attribute in SAML assertion")
	}

	// Find or create user by email
	user, err := s.userRepo.GetByEmail(ctx, attrs.Email)
	if err != nil {
		if err == repo.ErrNotFound {
			// Create new user via SAML
			name := attrs.Name
			user = &models.User{
				Email:      attrs.Email,
				Name:       &name,
				Provider:   string(models.AuthProviderSAML),
				ProviderID: attrs.Email, // Use email as provider ID for SAML
			}
			if err := s.userRepo.Create(ctx, user); err != nil {
				return nil, "", fmt.Errorf("create user: %w", err)
			}
		} else {
			return nil, "", fmt.Errorf("get user by email: %w", err)
		}
	}

	// Check if user is deactivated
	if user.DeactivatedAt != nil {
		return nil, "", fmt.Errorf("user account is deactivated")
	}

	// JIT: ensure user is a member of the team
	_, err = s.teamRepo.GetMember(ctx, team.ID, user.ID)
	if err != nil {
		if err == repo.ErrNotFound {
			if err := s.teamRepo.AddMember(ctx, team.ID, user.ID, models.RoleMember, nil); err != nil {
				return nil, "", fmt.Errorf("add team member: %w", err)
			}
		} else {
			return nil, "", fmt.Errorf("get team member: %w", err)
		}
	}

	return user, state.RedirectURL, nil
}

// IsSSOMandatory checks if SSO is mandatory for a team.
func (s *Service) IsSSOMandatory(ctx context.Context, teamID uuid.UUID) (bool, error) {
	conn, err := s.ssoRepo.GetConnectionByTeamID(ctx, teamID)
	if err != nil {
		if err == repo.ErrNotFound {
			return false, nil
		}
		return false, err
	}
	return conn.EnforceSSO, nil
}
