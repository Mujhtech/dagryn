package grpc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mujhtech/dagryn/pkg/dagryn/cache"

	repb "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// maxBatchSize is the threshold for using BatchUpdateBlobs / BatchReadBlobs
// vs ByteStream for individual blobs. REAPI recommends 4 MiB.
const maxBatchSize = 4 << 20 // 4 MiB

// Backend implements cache.Backend using the Bazel Remote Execution API v2.
// It uses the ActionCache, CAS, and ByteStream services.
type Backend struct {
	conn         *grpc.ClientConn
	instanceName string
	projectRoot  string
	actionCache  repb.ActionCacheClient
	cas          repb.ContentAddressableStorageClient
	byteStream   *byteStreamClient
}

// NewBackend creates a gRPC cache backend connected to a REAPI server.
func NewBackend(conn *grpc.ClientConn, instanceName, projectRoot string) *Backend {
	return &Backend{
		conn:         conn,
		instanceName: instanceName,
		projectRoot:  projectRoot,
		actionCache:  repb.NewActionCacheClient(conn),
		cas:          repb.NewContentAddressableStorageClient(conn),
		byteStream:   newByteStreamClient(conn),
	}
}

// Close closes the underlying gRPC connection.
func (b *Backend) Close() error {
	return b.conn.Close()
}

// actionDigest computes the REAPI action digest from task name and cache key.
// The digest is SHA256(taskName + "\x00" + cacheKey).
func actionDigest(taskName, cacheKey string) *repb.Digest {
	data := []byte(taskName + "\x00" + cacheKey)
	h := sha256.Sum256(data)
	return &repb.Digest{
		Hash:      hex.EncodeToString(h[:]),
		SizeBytes: int64(len(data)),
	}
}

func (b *Backend) Check(ctx context.Context, taskName, key string) (bool, error) {
	_, err := b.actionCache.GetActionResult(ctx, &repb.GetActionResultRequest{
		InstanceName:   b.instanceName,
		ActionDigest:   actionDigest(taskName, key),
		DigestFunction: repb.DigestFunction_SHA256,
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return false, nil
		}
		return false, fmt.Errorf("grpc cache check: %w", err)
	}
	return true, nil
}

func (b *Backend) Restore(ctx context.Context, taskName, key string) error {
	result, err := b.actionCache.GetActionResult(ctx, &repb.GetActionResultRequest{
		InstanceName:   b.instanceName,
		ActionDigest:   actionDigest(taskName, key),
		DigestFunction: repb.DigestFunction_SHA256,
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return fmt.Errorf("grpc cache not found for task %q key %q", taskName, key)
		}
		return fmt.Errorf("grpc cache restore: %w", err)
	}

	for _, of := range result.GetOutputFiles() {
		data, err := b.downloadBlob(ctx, of.GetDigest())
		if err != nil {
			return fmt.Errorf("grpc cache restore %q: %w", of.GetPath(), err)
		}

		dest := filepath.Join(b.projectRoot, of.GetPath())
		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return fmt.Errorf("grpc cache restore mkdir %q: %w", of.GetPath(), err)
		}

		perm := os.FileMode(0644)
		if of.GetIsExecutable() {
			perm = 0755
		}
		if err := os.WriteFile(dest, data, perm); err != nil {
			return fmt.Errorf("grpc cache restore write %q: %w", of.GetPath(), err)
		}
	}
	return nil
}

func (b *Backend) Save(ctx context.Context, taskName, key string, outputPatterns []string, _ cache.Metadata) error {
	// Collect files and compute digests.
	resolved, err := cache.ResolveFilePatterns(b.projectRoot, outputPatterns)
	if err != nil {
		return fmt.Errorf("grpc cache resolve patterns: %w", err)
	}

	var files []fileEntry
	for _, src := range resolved {
		info, err := os.Stat(src)
		if err != nil {
			continue
		}
		relPath, err := filepath.Rel(b.projectRoot, src)
		if err != nil {
			continue
		}

		data, err := os.ReadFile(src)
		if err != nil {
			continue
		}

		h := sha256.Sum256(data)
		files = append(files, fileEntry{
			relPath: relPath,
			data:    data,
			digest: &repb.Digest{
				Hash:      hex.EncodeToString(h[:]),
				SizeBytes: int64(len(data)),
			},
			isExecutable: info.Mode()&0111 != 0,
		})
	}

	// Find which blobs are missing from CAS.
	if len(files) > 0 {
		blobDigests := make([]*repb.Digest, len(files))
		for i, f := range files {
			blobDigests[i] = f.digest
		}

		missingResp, err := b.cas.FindMissingBlobs(ctx, &repb.FindMissingBlobsRequest{
			InstanceName:   b.instanceName,
			BlobDigests:    blobDigests,
			DigestFunction: repb.DigestFunction_SHA256,
		})
		if err != nil {
			return fmt.Errorf("grpc cache find missing: %w", err)
		}

		missingSet := make(map[string]bool, len(missingResp.GetMissingBlobDigests()))
		for _, d := range missingResp.GetMissingBlobDigests() {
			missingSet[d.GetHash()] = true
		}

		// Upload missing blobs.
		if err := b.uploadMissing(ctx, files, missingSet); err != nil {
			return err
		}
	}

	// Build ActionResult and store in ActionCache.
	outputFiles := make([]*repb.OutputFile, len(files))
	for i, f := range files {
		outputFiles[i] = &repb.OutputFile{
			Path:         f.relPath,
			Digest:       f.digest,
			IsExecutable: f.isExecutable,
		}
	}

	_, err = b.actionCache.UpdateActionResult(ctx, &repb.UpdateActionResultRequest{
		InstanceName:   b.instanceName,
		ActionDigest:   actionDigest(taskName, key),
		ActionResult:   &repb.ActionResult{OutputFiles: outputFiles},
		DigestFunction: repb.DigestFunction_SHA256,
	})
	if err != nil {
		return fmt.Errorf("grpc cache update action result: %w", err)
	}
	return nil
}

type fileEntry struct {
	relPath      string
	data         []byte
	digest       *repb.Digest
	isExecutable bool
}

func (b *Backend) uploadMissing(ctx context.Context, files []fileEntry, missingSet map[string]bool) error {
	// Separate into batch-eligible (≤4MB) and large blobs.
	var batchReqs []*repb.BatchUpdateBlobsRequest_Request
	type largeBlob struct {
		hash      string
		sizeBytes int64
		data      []byte
	}
	var largeBlobs []largeBlob

	for _, f := range files {
		if !missingSet[f.digest.GetHash()] {
			continue
		}
		if f.digest.GetSizeBytes() <= maxBatchSize {
			batchReqs = append(batchReqs, &repb.BatchUpdateBlobsRequest_Request{
				Digest: f.digest,
				Data:   f.data,
			})
		} else {
			largeBlobs = append(largeBlobs, largeBlob{
				hash:      f.digest.GetHash(),
				sizeBytes: f.digest.GetSizeBytes(),
				data:      f.data,
			})
		}
	}

	// Batch upload small blobs.
	if len(batchReqs) > 0 {
		resp, err := b.cas.BatchUpdateBlobs(ctx, &repb.BatchUpdateBlobsRequest{
			InstanceName:   b.instanceName,
			Requests:       batchReqs,
			DigestFunction: repb.DigestFunction_SHA256,
		})
		if err != nil {
			return fmt.Errorf("grpc cache batch upload: %w", err)
		}
		for _, r := range resp.GetResponses() {
			if s := r.GetStatus(); s != nil && s.GetCode() != int32(codes.OK) {
				return fmt.Errorf("grpc cache batch upload blob %s: %s", r.GetDigest().GetHash()[:8], s.GetMessage())
			}
		}
	}

	// Stream upload large blobs.
	for _, lb := range largeBlobs {
		if err := uploadBlobByteStream(ctx, b.byteStream, b.instanceName, lb.hash, lb.sizeBytes, lb.data); err != nil {
			return fmt.Errorf("grpc cache upload large blob: %w", err)
		}
	}

	return nil
}

// downloadBlob downloads a blob from CAS, choosing batch or bytestream based on size.
func (b *Backend) downloadBlob(ctx context.Context, digest *repb.Digest) ([]byte, error) {
	if digest.GetSizeBytes() <= maxBatchSize {
		return b.downloadBlobBatch(ctx, digest)
	}
	return downloadBlobByteStream(ctx, b.byteStream, b.instanceName, digest.GetHash(), digest.GetSizeBytes())
}

func (b *Backend) downloadBlobBatch(ctx context.Context, digest *repb.Digest) ([]byte, error) {
	resp, err := b.cas.BatchReadBlobs(ctx, &repb.BatchReadBlobsRequest{
		InstanceName:   b.instanceName,
		Digests:        []*repb.Digest{digest},
		DigestFunction: repb.DigestFunction_SHA256,
	})
	if err != nil {
		return nil, fmt.Errorf("grpc cache batch read: %w", err)
	}

	responses := resp.GetResponses()
	if len(responses) == 0 {
		return nil, fmt.Errorf("grpc cache batch read: empty response for %s", digest.GetHash()[:8])
	}
	r := responses[0]
	if s := r.GetStatus(); s != nil && s.GetCode() != int32(codes.OK) {
		return nil, fmt.Errorf("grpc cache batch read blob %s: %s", digest.GetHash()[:8], s.GetMessage())
	}
	return r.GetData(), nil
}

// Clear is a no-op for REAPI. Servers use TTL/LRU eviction.
func (b *Backend) Clear(_ context.Context, _ string) error {
	return nil
}

// ClearAll is a no-op for REAPI. Servers use TTL/LRU eviction.
func (b *Backend) ClearAll(_ context.Context) error {
	return nil
}

// Verify interface compliance at compile time.
var _ cache.Backend = (*Backend)(nil)
