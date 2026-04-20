package secrets

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

type cloudflareProvider struct {
	accountID string
	token     string
	baseURL   string
}

func (p *cloudflareProvider) Put(ctx context.Context, ref, value string) error {
	accountID := strings.TrimSpace(p.accountID)
	if accountID == "" {
		accountID = strings.TrimSpace(os.Getenv("CLOUDFLARE_ACCOUNT_ID"))
	}
	token := strings.TrimSpace(p.token)
	if token == "" {
		token = strings.TrimSpace(os.Getenv("CLOUDFLARE_API_TOKEN"))
	}
	if accountID == "" || token == "" {
		return errors.New("cloudflare provider requires CLOUDFLARE_ACCOUNT_ID and CLOUDFLARE_API_TOKEN")
	}

	storeID, secretName, err := parseCloudflareRef(ref)
	if err != nil {
		return err
	}

	secretID, err := p.findCloudflareSecretID(ctx, accountID, token, storeID, secretName)
	if err != nil {
		return err
	}

	if secretID == "" {
		return p.createCloudflareSecret(ctx, accountID, token, storeID, secretName, value)
	}

	return p.patchCloudflareSecret(ctx, accountID, token, storeID, secretID, value)
}

func (p *cloudflareProvider) Get(_ context.Context, _ string) (string, error) {
	return "", fmt.Errorf("cloudflare secrets provider does not support plaintext readback via API: %w", ErrUnsupportedProvider)
}

func (p *cloudflareProvider) Delete(ctx context.Context, ref string) error {
	accountID := strings.TrimSpace(p.accountID)
	if accountID == "" {
		accountID = strings.TrimSpace(os.Getenv("CLOUDFLARE_ACCOUNT_ID"))
	}
	token := strings.TrimSpace(p.token)
	if token == "" {
		token = strings.TrimSpace(os.Getenv("CLOUDFLARE_API_TOKEN"))
	}
	if accountID == "" || token == "" {
		return errors.New("cloudflare provider requires CLOUDFLARE_ACCOUNT_ID and CLOUDFLARE_API_TOKEN")
	}

	storeID, secretName, err := parseCloudflareRef(ref)
	if err != nil {
		return err
	}

	secretID, err := p.findCloudflareSecretID(ctx, accountID, token, storeID, secretName)
	if err != nil {
		return err
	}
	if secretID == "" {
		return nil
	}

	endpoint := fmt.Sprintf("%s/accounts/%s/secrets_store/stores/%s/secrets/%s", cloudflareAPIBaseURL(p.baseURL), url.PathEscape(accountID), url.PathEscape(storeID), url.PathEscape(secretID))
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return fmt.Errorf("cloudflare delete request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("cloudflare delete secret request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("cloudflare delete secret failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return nil
}

func parseCloudflareRef(ref string) (storeID string, secretName string, err error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", "", errors.New("cloudflare provider ref is required")
	}

	var parts []string
	if strings.Contains(ref, ":") {
		parts = strings.SplitN(ref, ":", 2)
	} else {
		parts = strings.SplitN(ref, "/", 2)
	}

	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", errors.New("invalid cloudflare provider_ref format; expected <store_id>:<secret_name>")
	}

	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
}

func (p *cloudflareProvider) createCloudflareSecret(ctx context.Context, accountID, token, storeID, secretName, value string) error {
	payload := []map[string]any{{
		"name":   secretName,
		"scopes": []string{"workers"},
		"value":  value,
	}}
	body, _ := json.Marshal(payload)

	endpoint := fmt.Sprintf("%s/accounts/%s/secrets_store/stores/%s/secrets", cloudflareAPIBaseURL(p.baseURL), url.PathEscape(accountID), url.PathEscape(storeID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("cloudflare create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("cloudflare create secret request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("cloudflare create secret failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return nil
}

func (p *cloudflareProvider) patchCloudflareSecret(ctx context.Context, accountID, token, storeID, secretID, value string) error {
	payload := map[string]any{
		"scopes": []string{"workers"},
		"value":  value,
	}
	body, _ := json.Marshal(payload)

	endpoint := fmt.Sprintf("%s/accounts/%s/secrets_store/stores/%s/secrets/%s", cloudflareAPIBaseURL(p.baseURL), url.PathEscape(accountID), url.PathEscape(storeID), url.PathEscape(secretID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("cloudflare patch request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("cloudflare patch secret request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("cloudflare patch secret failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return nil
}

func (p *cloudflareProvider) findCloudflareSecretID(ctx context.Context, accountID, token, storeID, secretName string) (string, error) {
	endpoint := fmt.Sprintf("%s/accounts/%s/secrets_store/stores/%s/secrets?search=%s&per_page=100", cloudflareAPIBaseURL(p.baseURL), url.PathEscape(accountID), url.PathEscape(storeID), url.QueryEscape(secretName))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("cloudflare list request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("cloudflare list secrets request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("cloudflare list secrets failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var parsed struct {
		Success bool `json:"success"`
		Result  []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", fmt.Errorf("cloudflare list decode failed: %w", err)
	}

	for _, item := range parsed.Result {
		if item.Name == secretName {
			return item.ID, nil
		}
	}
	return "", nil
}

func cloudflareAPIBaseURL(configured string) string {
	base := strings.TrimSpace(configured)
	if base == "" {
		base = strings.TrimSpace(os.Getenv("CLOUDFLARE_API_BASE_URL"))
	}
	if base == "" {
		return "https://api.cloudflare.com/client/v4"
	}
	return strings.TrimSuffix(base, "/")
}
