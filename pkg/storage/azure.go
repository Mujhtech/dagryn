package storage

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/service"
)

// AzureBucket implements Bucket using Azure Blob Storage.
type AzureBucket struct {
	client    *azblob.Client
	container string
	prefix    string
}

// NewAzureBucket creates an Azure Blob Storage-backed Bucket from the given Config.
// Config.Bucket is the container name, Config.Endpoint is the storage account URL.
// If AccessKeyID (account name) and SecretAccessKey (account key) are set, shared key
// credentials are used; otherwise falls back to DefaultAzureCredential.
func NewAzureBucket(cfg Config) (*AzureBucket, error) {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		return nil, fmt.Errorf("storage/azure: endpoint (storage account URL) is required")
	}

	var client *azblob.Client
	var err error

	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		cred, credErr := azblob.NewSharedKeyCredential(cfg.AccessKeyID, cfg.SecretAccessKey)
		if credErr != nil {
			return nil, fmt.Errorf("storage/azure: shared key credential: %w", credErr)
		}
		client, err = azblob.NewClientWithSharedKeyCredential(endpoint, cred, nil)
	} else {
		cred, credErr := azidentity.NewDefaultAzureCredential(nil)
		if credErr != nil {
			return nil, fmt.Errorf("storage/azure: default credential: %w", credErr)
		}
		client, err = azblob.NewClient(endpoint, cred, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("storage/azure: create client: %w", err)
	}

	return &AzureBucket{
		client:    client,
		container: cfg.Bucket,
		prefix:    cfg.Prefix,
	}, nil
}

func (b *AzureBucket) fullKey(key string) string {
	return b.prefix + key
}

func (b *AzureBucket) Put(ctx context.Context, key string, r io.Reader, opts *PutOptions) error {
	blobOpts := &azblob.UploadStreamOptions{}
	if opts != nil && opts.ContentType != "" {
		blobOpts.HTTPHeaders = &blob.HTTPHeaders{
			BlobContentType: &opts.ContentType,
		}
	}
	_, err := b.client.UploadStream(ctx, b.container, b.fullKey(key), r, blobOpts)
	if err != nil {
		return fmt.Errorf("storage/azure: put %q: %w", key, err)
	}
	return nil
}

func (b *AzureBucket) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	resp, err := b.client.DownloadStream(ctx, b.container, b.fullKey(key), nil)
	if err != nil {
		if isAzureNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("storage/azure: get %q: %w", key, err)
	}
	return resp.Body, nil
}

func (b *AzureBucket) Delete(ctx context.Context, key string) error {
	_, err := b.client.DeleteBlob(ctx, b.container, b.fullKey(key), nil)
	if err != nil {
		if isAzureNotFound(err) {
			return nil
		}
		return fmt.Errorf("storage/azure: delete %q: %w", key, err)
	}
	return nil
}

func (b *AzureBucket) Exists(ctx context.Context, key string) (bool, error) {
	pager := b.client.NewListBlobsFlatPager(b.container, &azblob.ListBlobsFlatOptions{
		Prefix:     strPtr(b.fullKey(key)),
		MaxResults: int32Ptr(1),
	})
	if pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return false, fmt.Errorf("storage/azure: exists %q: %w", key, err)
		}
		for _, item := range page.Segment.BlobItems {
			if *item.Name == b.fullKey(key) {
				return true, nil
			}
		}
	}
	return false, nil
}

func (b *AzureBucket) List(ctx context.Context, prefix string, opts *ListOptions) (*ListResult, error) {
	maxKeys := int32(1000)
	if opts != nil && opts.MaxKeys > 0 {
		maxKeys = int32(opts.MaxKeys)
	}

	pager := b.client.NewListBlobsFlatPager(b.container, &azblob.ListBlobsFlatOptions{
		Prefix:     strPtr(b.fullKey(prefix)),
		MaxResults: &maxKeys,
	})

	result := &ListResult{}
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("storage/azure: list prefix %q: %w", prefix, err)
		}
		for _, item := range page.Segment.BlobItems {
			key := strings.TrimPrefix(*item.Name, b.prefix)
			result.Keys = append(result.Keys, key)
		}
		if len(result.Keys) >= int(maxKeys) {
			result.IsTruncated = pager.More()
			break
		}
	}
	return result, nil
}

// SignedPutURL returns a pre-signed URL for uploading a blob.
func (b *AzureBucket) SignedPutURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	return b.generateSASURL(key, expiry, sas.BlobPermissions{Write: true, Create: true})
}

// SignedGetURL returns a pre-signed URL for downloading a blob.
func (b *AzureBucket) SignedGetURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	return b.generateSASURL(key, expiry, sas.BlobPermissions{Read: true})
}

func (b *AzureBucket) generateSASURL(key string, expiry time.Duration, perms sas.BlobPermissions) (string, error) {
	svcClient, err := service.NewClientFromConnectionString("", nil)
	if err != nil {
		// If no connection string, try using the existing client's SAS generation
		return "", fmt.Errorf("storage/azure: signed url requires shared key credential or connection string: %w", err)
	}

	now := time.Now().UTC()
	sasURL, err := svcClient.NewContainerClient(b.container).NewBlobClient(b.fullKey(key)).GetSASURL(perms, now.Add(expiry), nil)
	if err != nil {
		return "", fmt.Errorf("storage/azure: generate sas url %q: %w", key, err)
	}
	return sasURL, nil
}

func isAzureNotFound(err error) bool {
	return strings.Contains(err.Error(), "BlobNotFound") ||
		strings.Contains(err.Error(), "ContainerNotFound") ||
		strings.Contains(err.Error(), "404")
}

func strPtr(s string) *string { return &s }
func int32Ptr(i int32) *int32 { return &i }
