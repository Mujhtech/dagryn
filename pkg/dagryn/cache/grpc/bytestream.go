package grpc

import (
	"context"
	"fmt"
	"io"

	bspb "google.golang.org/genproto/googleapis/bytestream"
	"google.golang.org/grpc"
)

// ByteStream service path.
const byteStreamService = "/google.bytestream.ByteStream/"

// byteStreamClient wraps gRPC calls for the google.bytestream.ByteStream service.
// We implement this manually because the genproto package only provides message
// types, not the generated gRPC stubs.
type byteStreamClient struct {
	cc grpc.ClientConnInterface
}

func newByteStreamClient(cc grpc.ClientConnInterface) *byteStreamClient {
	return &byteStreamClient{cc: cc}
}

type byteStreamReadClient struct {
	grpc.ClientStream
}

func (x *byteStreamReadClient) Recv() (*bspb.ReadResponse, error) {
	m := new(bspb.ReadResponse)
	if err := x.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// Read opens a server-streaming Read RPC.
func (c *byteStreamClient) Read(ctx context.Context, in *bspb.ReadRequest, opts ...grpc.CallOption) (byteStreamReader, error) {
	desc := &grpc.StreamDesc{
		StreamName:    "Read",
		ServerStreams: true,
	}
	stream, err := c.cc.NewStream(ctx, desc, byteStreamService+"Read", opts...)
	if err != nil {
		return nil, err
	}
	if err := stream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := stream.CloseSend(); err != nil {
		return nil, err
	}
	return &byteStreamReadClient{stream}, nil
}

type byteStreamReader interface {
	Recv() (*bspb.ReadResponse, error)
}

type byteStreamWriteClient struct {
	grpc.ClientStream
}

func (x *byteStreamWriteClient) Send(req *bspb.WriteRequest) error {
	return x.SendMsg(req)
}

func (x *byteStreamWriteClient) CloseAndRecv() (*bspb.WriteResponse, error) {
	if err := x.CloseSend(); err != nil {
		return nil, err
	}
	m := new(bspb.WriteResponse)
	if err := x.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

type byteStreamWriter interface {
	Send(*bspb.WriteRequest) error
	CloseAndRecv() (*bspb.WriteResponse, error)
}

// Write opens a client-streaming Write RPC.
func (c *byteStreamClient) Write(ctx context.Context, opts ...grpc.CallOption) (byteStreamWriter, error) {
	desc := &grpc.StreamDesc{
		StreamName:    "Write",
		ClientStreams: true,
	}
	stream, err := c.cc.NewStream(ctx, desc, byteStreamService+"Write", opts...)
	if err != nil {
		return nil, err
	}
	return &byteStreamWriteClient{stream}, nil
}

// downloadBlob reads a blob from ByteStream and returns all data.
func downloadBlobByteStream(ctx context.Context, bs *byteStreamClient, instanceName, hash string, sizeBytes int64) ([]byte, error) {
	resourceName := blobResourceName(instanceName, hash, sizeBytes)
	stream, err := bs.Read(ctx, &bspb.ReadRequest{ResourceName: resourceName})
	if err != nil {
		return nil, fmt.Errorf("bytestream read %s: %w", hash[:8], err)
	}

	buf := make([]byte, 0, sizeBytes)
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("bytestream recv %s: %w", hash[:8], err)
		}
		buf = append(buf, resp.GetData()...)
	}
	return buf, nil
}

// uploadBlobByteStream writes a blob to ByteStream in chunks.
func uploadBlobByteStream(ctx context.Context, bs *byteStreamClient, instanceName, hash string, sizeBytes int64, data []byte) error {
	resourceName := uploadResourceName(instanceName, hash, sizeBytes)
	stream, err := bs.Write(ctx)
	if err != nil {
		return fmt.Errorf("bytestream write %s: %w", hash[:8], err)
	}

	const chunkSize = 1 << 20 // 1 MiB
	offset := int64(0)
	for offset < int64(len(data)) {
		end := offset + chunkSize
		if end > int64(len(data)) {
			end = int64(len(data))
		}
		req := &bspb.WriteRequest{
			ResourceName: resourceName,
			WriteOffset:  offset,
			Data:         data[offset:end],
			FinishWrite:  end == int64(len(data)),
		}
		if err := stream.Send(req); err != nil {
			return fmt.Errorf("bytestream send %s: %w", hash[:8], err)
		}
		offset = end
	}

	if _, err := stream.CloseAndRecv(); err != nil {
		return fmt.Errorf("bytestream close %s: %w", hash[:8], err)
	}
	return nil
}

// blobResourceName returns the download resource name: {instance}/blobs/{hash}/{size}
func blobResourceName(instanceName, hash string, sizeBytes int64) string {
	if instanceName == "" {
		return fmt.Sprintf("blobs/%s/%d", hash, sizeBytes)
	}
	return fmt.Sprintf("%s/blobs/%s/%d", instanceName, hash, sizeBytes)
}

// uploadResourceName returns the upload resource name: {instance}/uploads/{uuid}/blobs/{hash}/{size}
func uploadResourceName(instanceName, hash string, sizeBytes int64) string {
	// Use a prefix of the hash as a pseudo-UUID for the upload.
	uuid := hash[:8]
	if instanceName == "" {
		return fmt.Sprintf("uploads/%s/blobs/%s/%d", uuid, hash, sizeBytes)
	}
	return fmt.Sprintf("%s/uploads/%s/blobs/%s/%d", instanceName, uuid, hash, sizeBytes)
}
