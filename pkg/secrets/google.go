package secrets

import (
	"context"
	"errors"
	"fmt"
	"strings"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
)

type gcpProvider struct {
	client *secretmanager.Client
}

func (p *gcpProvider) Put(ctx context.Context, ref, value string) error {
	name := normalizeGCPSecretRef(ref)
	_, err := p.client.AddSecretVersion(ctx, &secretmanagerpb.AddSecretVersionRequest{
		Parent: name,
		Payload: &secretmanagerpb.SecretPayload{
			Data: []byte(value),
		},
	})
	if err != nil {
		return fmt.Errorf("add gcp secret version: %w", err)
	}
	return nil
}

func (p *gcpProvider) Get(ctx context.Context, ref string) (string, error) {
	name := normalizeGCPSecretVersionRef(ref)
	out, err := p.client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{Name: name})
	if err != nil {
		return "", fmt.Errorf("access gcp secret version: %w", err)
	}
	if out.Payload == nil {
		return "", errors.New("gcp secret payload is empty")
	}
	return string(out.Payload.Data), nil
}

func (p *gcpProvider) Delete(ctx context.Context, ref string) error {
	name := normalizeGCPSecretRef(ref)
	err := p.client.DeleteSecret(ctx, &secretmanagerpb.DeleteSecretRequest{Name: name})
	if err != nil {
		return fmt.Errorf("delete gcp secret: %w", err)
	}
	return nil
}

func normalizeGCPSecretRef(ref string) string {
	ref = strings.TrimSpace(ref)
	if strings.Contains(ref, "/versions/") {
		parts := strings.Split(ref, "/versions/")
		return parts[0]
	}
	return ref
}

func normalizeGCPSecretVersionRef(ref string) string {
	ref = strings.TrimSpace(ref)
	if strings.Contains(ref, "/versions/") {
		return ref
	}
	return ref + "/versions/latest"
}
