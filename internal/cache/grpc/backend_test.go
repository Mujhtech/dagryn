package grpc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"

	repb "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/mujhtech/dagryn/internal/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	bspb "google.golang.org/genproto/googleapis/bytestream"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// fakeREAPIServer implements the REAPI services for testing.
type fakeREAPIServer struct {
	repb.UnimplementedActionCacheServer
	repb.UnimplementedContentAddressableStorageServer

	mu          sync.Mutex
	actionCache map[string]*repb.ActionResult // keyed by digest hash
	cas         map[string][]byte             // keyed by digest hash
}

func newFakeServer() *fakeREAPIServer {
	return &fakeREAPIServer{
		actionCache: make(map[string]*repb.ActionResult),
		cas:         make(map[string][]byte),
	}
}

func (s *fakeREAPIServer) GetActionResult(_ context.Context, req *repb.GetActionResultRequest) (*repb.ActionResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := req.GetActionDigest().GetHash()
	if req.GetInstanceName() != "" {
		key = req.GetInstanceName() + "/" + key
	}
	result, ok := s.actionCache[key]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "action result not found")
	}
	return result, nil
}

func (s *fakeREAPIServer) UpdateActionResult(_ context.Context, req *repb.UpdateActionResultRequest) (*repb.ActionResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := req.GetActionDigest().GetHash()
	if req.GetInstanceName() != "" {
		key = req.GetInstanceName() + "/" + key
	}
	s.actionCache[key] = req.GetActionResult()
	return req.GetActionResult(), nil
}

func (s *fakeREAPIServer) FindMissingBlobs(_ context.Context, req *repb.FindMissingBlobsRequest) (*repb.FindMissingBlobsResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var missing []*repb.Digest
	for _, d := range req.GetBlobDigests() {
		if _, ok := s.cas[d.GetHash()]; !ok {
			missing = append(missing, d)
		}
	}
	return &repb.FindMissingBlobsResponse{MissingBlobDigests: missing}, nil
}

func (s *fakeREAPIServer) BatchUpdateBlobs(_ context.Context, req *repb.BatchUpdateBlobsRequest) (*repb.BatchUpdateBlobsResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var responses []*repb.BatchUpdateBlobsResponse_Response
	for _, r := range req.GetRequests() {
		s.cas[r.GetDigest().GetHash()] = r.GetData()
		responses = append(responses, &repb.BatchUpdateBlobsResponse_Response{
			Digest: r.GetDigest(),
		})
	}
	return &repb.BatchUpdateBlobsResponse{Responses: responses}, nil
}

func (s *fakeREAPIServer) BatchReadBlobs(_ context.Context, req *repb.BatchReadBlobsRequest) (*repb.BatchReadBlobsResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var responses []*repb.BatchReadBlobsResponse_Response
	for _, d := range req.GetDigests() {
		data, ok := s.cas[d.GetHash()]
		if !ok {
			responses = append(responses, &repb.BatchReadBlobsResponse_Response{
				Digest: d,
				Status: status.New(codes.NotFound, "not found").Proto(),
			})
			continue
		}
		responses = append(responses, &repb.BatchReadBlobsResponse_Response{
			Digest: d,
			Data:   data,
		})
	}
	return &repb.BatchReadBlobsResponse{Responses: responses}, nil
}

// fakeByteStreamServer implements the ByteStream service for testing.
type fakeByteStreamServer struct {
	bspb.UnimplementedByteStreamServer
	fake *fakeREAPIServer
}

func (s *fakeByteStreamServer) Read(req *bspb.ReadRequest, stream bspb.ByteStream_ReadServer) error {
	// Parse resource name: [instance/]blobs/{hash}/{size}
	hash := extractHashFromResource(req.GetResourceName())
	s.fake.mu.Lock()
	data, ok := s.fake.cas[hash]
	s.fake.mu.Unlock()

	if !ok {
		return status.Errorf(codes.NotFound, "blob %s not found", hash[:8])
	}

	// Send in 1MB chunks to simulate streaming.
	const chunkSize = 1 << 20
	for offset := 0; offset < len(data); offset += chunkSize {
		end := offset + chunkSize
		if end > len(data) {
			end = len(data)
		}
		if err := stream.Send(&bspb.ReadResponse{Data: data[offset:end]}); err != nil {
			return err
		}
	}
	return nil
}

func (s *fakeByteStreamServer) Write(stream bspb.ByteStream_WriteServer) error {
	var allData []byte
	var resourceName string

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if resourceName == "" {
			resourceName = req.GetResourceName()
		}
		allData = append(allData, req.GetData()...)
		if req.GetFinishWrite() {
			break
		}
	}

	hash := extractHashFromUploadResource(resourceName)
	s.fake.mu.Lock()
	s.fake.cas[hash] = allData
	s.fake.mu.Unlock()

	return stream.SendAndClose(&bspb.WriteResponse{CommittedSize: int64(len(allData))})
}

// extractHashFromResource parses: [instance/]blobs/{hash}/{size}
func extractHashFromResource(resource string) string {
	// Walk from end: /blobs/{hash}/{size}
	parts := splitPath(resource)
	for i, p := range parts {
		if p == "blobs" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

// extractHashFromUploadResource parses: [instance/]uploads/{uuid}/blobs/{hash}/{size}
func extractHashFromUploadResource(resource string) string {
	return extractHashFromResource(resource)
}

func splitPath(s string) []string {
	var parts []string
	for s != "" {
		i := 0
		for i < len(s) && s[i] != '/' {
			i++
		}
		if i > 0 {
			parts = append(parts, s[:i])
		}
		if i < len(s) {
			s = s[i+1:]
		} else {
			break
		}
	}
	return parts
}

// setupTest creates a fake REAPI server, in-process gRPC connection, and Backend.
func setupTest(t *testing.T) (*Backend, *fakeREAPIServer) {
	t.Helper()

	fake := newFakeServer()
	lis := bufconn.Listen(1024 * 1024)

	srv := grpc.NewServer()
	repb.RegisterActionCacheServer(srv, fake)
	repb.RegisterContentAddressableStorageServer(srv, fake)
	bspb.RegisterByteStreamServer(srv, &fakeByteStreamServer{fake: fake})

	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(func() { srv.Stop() })

	conn, err := grpc.NewClient("passthrough:///bufconn",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	projectRoot := t.TempDir()
	backend := NewBackend(conn, "", projectRoot)
	return backend, fake
}

// setupTestWithInstance creates a backend with a non-empty instance name.
func setupTestWithInstance(t *testing.T, instanceName string) (*Backend, *fakeREAPIServer) {
	t.Helper()

	fake := newFakeServer()
	lis := bufconn.Listen(1024 * 1024)

	srv := grpc.NewServer()
	repb.RegisterActionCacheServer(srv, fake)
	repb.RegisterContentAddressableStorageServer(srv, fake)
	bspb.RegisterByteStreamServer(srv, &fakeByteStreamServer{fake: fake})

	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(func() { srv.Stop() })

	conn, err := grpc.NewClient("passthrough:///bufconn",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	projectRoot := t.TempDir()
	backend := NewBackend(conn, instanceName, projectRoot)
	return backend, fake
}

func TestGRPCBackend_CheckMiss(t *testing.T) {
	b, _ := setupTest(t)
	ctx := context.Background()

	hit, err := b.Check(ctx, "build", "abc123")
	require.NoError(t, err)
	assert.False(t, hit)
}

func TestGRPCBackend_CheckHit(t *testing.T) {
	b, _ := setupTest(t)
	ctx := context.Background()

	// Create a file in the project root.
	writeTestFile(t, b.projectRoot, "output.txt", "hello world", 0644)

	// Save it.
	err := b.Save(ctx, "build", "key1", []string{"output.txt"}, dummyMeta())
	require.NoError(t, err)

	// Check should now return true.
	hit, err := b.Check(ctx, "build", "key1")
	require.NoError(t, err)
	assert.True(t, hit)
}

func TestGRPCBackend_SaveAndRestore(t *testing.T) {
	b, _ := setupTest(t)
	ctx := context.Background()

	// Create test files.
	writeTestFile(t, b.projectRoot, "src/main.go", "package main\n", 0644)
	writeTestFile(t, b.projectRoot, "build/app", "binary content", 0755)

	// Save.
	err := b.Save(ctx, "build", "key1", []string{"src/main.go", "build/app"}, dummyMeta())
	require.NoError(t, err)

	// Delete original files.
	require.NoError(t, os.RemoveAll(filepath.Join(b.projectRoot, "src")))
	require.NoError(t, os.RemoveAll(filepath.Join(b.projectRoot, "build")))

	// Restore.
	err = b.Restore(ctx, "build", "key1")
	require.NoError(t, err)

	// Verify contents.
	data, err := os.ReadFile(filepath.Join(b.projectRoot, "src/main.go"))
	require.NoError(t, err)
	assert.Equal(t, "package main\n", string(data))

	data, err = os.ReadFile(filepath.Join(b.projectRoot, "build/app"))
	require.NoError(t, err)
	assert.Equal(t, "binary content", string(data))
}

func TestGRPCBackend_LargeBlobByteStream(t *testing.T) {
	b, fake := setupTest(t)
	ctx := context.Background()

	// Create a file larger than 4MB to force ByteStream usage.
	largeData := make([]byte, 5<<20) // 5 MiB
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}
	writeTestFileBytes(t, b.projectRoot, "large.bin", largeData, 0644)

	// Save.
	err := b.Save(ctx, "build", "large-key", []string{"large.bin"}, dummyMeta())
	require.NoError(t, err)

	// Verify the blob is in CAS.
	h := sha256.Sum256(largeData)
	hash := hex.EncodeToString(h[:])
	fake.mu.Lock()
	_, exists := fake.cas[hash]
	fake.mu.Unlock()
	assert.True(t, exists, "large blob should be in CAS via ByteStream")

	// Delete and restore.
	require.NoError(t, os.Remove(filepath.Join(b.projectRoot, "large.bin")))

	err = b.Restore(ctx, "build", "large-key")
	require.NoError(t, err)

	restored, err := os.ReadFile(filepath.Join(b.projectRoot, "large.bin"))
	require.NoError(t, err)
	assert.Equal(t, largeData, restored)
}

func TestGRPCBackend_FindMissingBlobs(t *testing.T) {
	b, fake := setupTest(t)
	ctx := context.Background()

	content := "shared content"
	writeTestFile(t, b.projectRoot, "file1.txt", content, 0644)

	// First save uploads the blob.
	err := b.Save(ctx, "task1", "key1", []string{"file1.txt"}, dummyMeta())
	require.NoError(t, err)

	h := sha256.Sum256([]byte(content))
	hash := hex.EncodeToString(h[:])

	fake.mu.Lock()
	blobCount := len(fake.cas)
	_, blobExists := fake.cas[hash]
	fake.mu.Unlock()
	assert.True(t, blobExists)

	// Second save with same content should skip upload (dedup via FindMissingBlobs).
	err = b.Save(ctx, "task2", "key2", []string{"file1.txt"}, dummyMeta())
	require.NoError(t, err)

	fake.mu.Lock()
	newBlobCount := len(fake.cas)
	fake.mu.Unlock()
	assert.Equal(t, blobCount, newBlobCount, "no new blobs should be uploaded")
}

func TestGRPCBackend_ExecutablePermission(t *testing.T) {
	b, _ := setupTest(t)
	ctx := context.Background()

	// Create an executable file.
	writeTestFile(t, b.projectRoot, "run.sh", "#!/bin/bash\necho hello\n", 0755)

	// Save.
	err := b.Save(ctx, "build", "exec-key", []string{"run.sh"}, dummyMeta())
	require.NoError(t, err)

	// Remove and restore.
	require.NoError(t, os.Remove(filepath.Join(b.projectRoot, "run.sh")))

	err = b.Restore(ctx, "build", "exec-key")
	require.NoError(t, err)

	// Verify executable permission is preserved.
	info, err := os.Stat(filepath.Join(b.projectRoot, "run.sh"))
	require.NoError(t, err)
	assert.True(t, info.Mode()&0111 != 0, "file should be executable")
}

func TestGRPCBackend_ClearNoop(t *testing.T) {
	b, _ := setupTest(t)
	ctx := context.Background()

	// Clear and ClearAll should be no-ops (no errors).
	assert.NoError(t, b.Clear(ctx, "build"))
	assert.NoError(t, b.ClearAll(ctx))
}

func TestGRPCBackend_InstanceName(t *testing.T) {
	instanceName := "dagryn-prod"
	b, fake := setupTestWithInstance(t, instanceName)
	ctx := context.Background()

	writeTestFile(t, b.projectRoot, "output.txt", "data", 0644)

	// Save with instance name.
	err := b.Save(ctx, "build", "inst-key", []string{"output.txt"}, dummyMeta())
	require.NoError(t, err)

	// Verify the action result is stored under instance-prefixed key.
	ad := actionDigest("build", "inst-key")
	expectedKey := fmt.Sprintf("%s/%s", instanceName, ad.GetHash())

	fake.mu.Lock()
	_, exists := fake.actionCache[expectedKey]
	fake.mu.Unlock()
	assert.True(t, exists, "action result should be stored with instance name prefix")

	// Check should find it.
	hit, err := b.Check(ctx, "build", "inst-key")
	require.NoError(t, err)
	assert.True(t, hit)
}

// --- helpers ---

func writeTestFile(t *testing.T, root, relPath, content string, perm os.FileMode) {
	t.Helper()
	p := filepath.Join(root, relPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0755))
	require.NoError(t, os.WriteFile(p, []byte(content), perm))
}

func writeTestFileBytes(t *testing.T, root, relPath string, data []byte, perm os.FileMode) {
	t.Helper()
	p := filepath.Join(root, relPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0755))
	require.NoError(t, os.WriteFile(p, data, perm))
}

func dummyMeta() cache.Metadata {
	return cache.Metadata{TaskName: "test"}
}
