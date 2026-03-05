package sso

import (
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/crewjam/saml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateSPCertificate(t *testing.T) {
	certPEM, keyPEM, err := GenerateSPCertificate()
	require.NoError(t, err)
	require.NotEmpty(t, certPEM)
	require.NotEmpty(t, keyPEM)

	// Verify cert is valid PEM
	certBlock, _ := pem.Decode(certPEM)
	require.NotNil(t, certBlock, "should decode certificate PEM")
	assert.Equal(t, "CERTIFICATE", certBlock.Type)

	// Parse the certificate
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	require.NoError(t, err)
	assert.Equal(t, "Dagryn SAML SP", cert.Subject.CommonName)
	assert.Contains(t, cert.Subject.Organization, "Dagryn")
	assert.True(t, cert.KeyUsage&x509.KeyUsageDigitalSignature != 0)
	assert.True(t, cert.KeyUsage&x509.KeyUsageKeyEncipherment != 0)

	// Verify key is valid PEM
	keyBlock, _ := pem.Decode(keyPEM)
	require.NotNil(t, keyBlock, "should decode key PEM")
	assert.Equal(t, "RSA PRIVATE KEY", keyBlock.Type)
}

func TestParseIDPMetadataXML_EntityDescriptor(t *testing.T) {
	xml := []byte(`<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="https://idp.example.com">
		<IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
			<SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://idp.example.com/sso"/>
		</IDPSSODescriptor>
	</EntityDescriptor>`)

	metadata, err := ParseIDPMetadataXML(xml)
	require.NoError(t, err)
	assert.Equal(t, "https://idp.example.com", metadata.EntityID)
	require.Len(t, metadata.IDPSSODescriptors, 1)
}

func TestParseIDPMetadataXML_EntitiesDescriptor(t *testing.T) {
	xml := []byte(`<EntitiesDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata">
		<EntityDescriptor entityID="https://idp.example.com">
			<IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
				<SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://idp.example.com/sso"/>
			</IDPSSODescriptor>
		</EntityDescriptor>
	</EntitiesDescriptor>`)

	metadata, err := ParseIDPMetadataXML(xml)
	require.NoError(t, err)
	assert.Equal(t, "https://idp.example.com", metadata.EntityID)
}

func TestParseIDPMetadataXML_InvalidXML(t *testing.T) {
	_, err := ParseIDPMetadataXML([]byte("not xml"))
	assert.Error(t, err)
}

func TestExtractAttributes(t *testing.T) {
	t.Run("extracts email and name from standard attributes", func(t *testing.T) {
		assertion := &saml.Assertion{
			AttributeStatements: []saml.AttributeStatement{
				{
					Attributes: []saml.Attribute{
						{
							Name: "email",
							Values: []saml.AttributeValue{
								{Value: "user@example.com"},
							},
						},
						{
							Name: "displayName",
							Values: []saml.AttributeValue{
								{Value: "Test User"},
							},
						},
						{
							Name: "groups",
							Values: []saml.AttributeValue{
								{Value: "admins"},
								{Value: "developers"},
							},
						},
					},
				},
			},
		}

		attrs := ExtractAttributes(assertion)
		assert.Equal(t, "user@example.com", attrs.Email)
		assert.Equal(t, "Test User", attrs.Name)
		assert.Equal(t, []string{"admins", "developers"}, attrs.Groups)
	})

	t.Run("extracts from claims-based attribute names", func(t *testing.T) {
		assertion := &saml.Assertion{
			AttributeStatements: []saml.AttributeStatement{
				{
					Attributes: []saml.Attribute{
						{
							Name: "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress",
							Values: []saml.AttributeValue{
								{Value: "claims@example.com"},
							},
						},
					},
				},
			},
		}

		attrs := ExtractAttributes(assertion)
		assert.Equal(t, "claims@example.com", attrs.Email)
	})

	t.Run("falls back to NameID for email", func(t *testing.T) {
		assertion := &saml.Assertion{
			Subject: &saml.Subject{
				NameID: &saml.NameID{
					Format: "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress",
					Value:  "nameid@example.com",
				},
			},
			AttributeStatements: []saml.AttributeStatement{
				{Attributes: []saml.Attribute{}},
			},
		}

		attrs := ExtractAttributes(assertion)
		assert.Equal(t, "nameid@example.com", attrs.Email)
	})

	t.Run("nil assertion returns empty", func(t *testing.T) {
		attrs := ExtractAttributes(nil)
		assert.Empty(t, attrs.Email)
		assert.Empty(t, attrs.Name)
		assert.Nil(t, attrs.Groups)
	})

	t.Run("nil attribute statements returns empty", func(t *testing.T) {
		attrs := ExtractAttributes(&saml.Assertion{})
		assert.Empty(t, attrs.Email)
	})
}
