package sso

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"encoding/xml"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
)

// SAMLUserAttributes holds the extracted attributes from a SAML assertion.
type SAMLUserAttributes struct {
	Email  string
	Name   string
	Groups []string
}

// ParseIDPMetadataURL fetches and parses IdP metadata from a URL.
func ParseIDPMetadataURL(metadataURL string) (*saml.EntityDescriptor, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	metadata, err := samlsp.FetchMetadata(
		context.Background(),
		client,
		*mustParseURL(metadataURL),
	)
	if err != nil {
		return nil, fmt.Errorf("fetch idp metadata: %w", err)
	}
	return metadata, nil
}

// ParseIDPMetadataXML parses IdP metadata from raw XML bytes.
func ParseIDPMetadataXML(data []byte) (*saml.EntityDescriptor, error) {
	var metadata saml.EntityDescriptor
	if err := xml.Unmarshal(data, &metadata); err != nil {
		// Try parsing as EntitiesDescriptor (some IdPs wrap in this)
		var entities saml.EntitiesDescriptor
		if err2 := xml.Unmarshal(data, &entities); err2 != nil {
			return nil, fmt.Errorf("parse idp metadata xml: %w", err)
		}
		if len(entities.EntityDescriptors) == 0 {
			return nil, fmt.Errorf("no entity descriptors found in metadata")
		}
		return &entities.EntityDescriptors[0], nil
	}
	return &metadata, nil
}

// ExtractAttributes extracts user attributes from a SAML assertion.
func ExtractAttributes(assertion *saml.Assertion) SAMLUserAttributes {
	attrs := SAMLUserAttributes{}
	if assertion == nil || assertion.AttributeStatements == nil {
		return attrs
	}

	for _, stmt := range assertion.AttributeStatements {
		for _, attr := range stmt.Attributes {
			values := make([]string, 0, len(attr.Values))
			for _, v := range attr.Values {
				values = append(values, v.Value)
			}
			if len(values) == 0 {
				continue
			}

			switch attr.Name {
			case "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress",
				"email", "Email", "mail":
				attrs.Email = values[0]
			case "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name",
				"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/givenname",
				"displayName", "name", "Name":
				attrs.Name = values[0]
			case "http://schemas.xmlsoap.org/claims/Group",
				"groups", "memberOf":
				attrs.Groups = values
			}
		}
	}

	// Fallback: use NameID as email if no email attribute found
	if attrs.Email == "" && assertion.Subject != nil && assertion.Subject.NameID != nil {
		if assertion.Subject.NameID.Format == "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress" ||
			assertion.Subject.NameID.Format == "" {
			attrs.Email = assertion.Subject.NameID.Value
		}
	}

	return attrs
}

// GenerateSPCertificate generates a self-signed X.509 certificate and RSA key pair for the SP.
func GenerateSPCertificate() (certPEM, keyPEM []byte, err error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("generate rsa key: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "Dagryn SAML SP",
			Organization: []string{"Dagryn"},
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour), // 10 years
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return nil, nil, fmt.Errorf("create certificate: %w", err)
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})

	return certPEM, keyPEM, nil
}
