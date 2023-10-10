// Copyright (2023 -- present) Cobalt Speech and Language Inc.

package client

import (
	"context"
	"fmt"
	"io"

	voicebiopb "github.com/cobaltspeech/go-genproto/cobaltspeech/voicebio/v1"
	"github.com/cobaltspeech/log"

	"google.golang.org/grpc"
)

const defaultStreamingBufsize uint32 = 1024

type Client struct {
	voicebio         voicebiopb.VoiceBioServiceClient
	conn             *grpc.ClientConn
	log              log.Logger
	streamingBufSize uint32
}

func NewClient(conn *grpc.ClientConn, opts ...Option) (*Client, error) {
	var args clientArgs

	args.streamingBufSize = defaultStreamingBufsize

	for _, opt := range opts {
		err := opt(&args)
		if err != nil {
			return nil, fmt.Errorf("unable to create a client: %v", err)
		}
	}

	if args.log == nil {
		args.log = log.NewDiscardLogger()
	}

	return &Client{
		voicebio:         voicebiopb.NewVoiceBioServiceClient(conn),
		conn:             conn,
		streamingBufSize: args.streamingBufSize,
		log:              args.log,
	}, nil
}

type clientArgs struct {
	log              log.Logger
	streamingBufSize uint32
}

// Option configures how we setup the connection with a server.
type Option func(*clientArgs) error

// WithStreamingBufferSize returns an Option that sets up the buffer size
// (bytes) of each message sent from the Client to the server during streaming
// GRPC calls.  Use this only if Cobalt recommends you to do so.  A value n>0 is
// required.
func WithStreamingBufferSize(n uint32) Option {
	return func(c *clientArgs) error {
		if n == 0 {
			return fmt.Errorf("invalid streaming buffer size of 0")
		}

		c.streamingBufSize = n

		return nil
	}
}

func WithLogger(logger log.Logger) Option {
	return func(c *clientArgs) error {
		c.log = logger
		return nil
	}
}

func (c *Client) CobaltVersions(ctx context.Context) (string, error) {
	v, err := c.voicebio.Version(ctx, &voicebiopb.VersionRequest{})
	if err != nil {
		return "", err
	}

	return v.Version, nil
}

func (c *Client) ListModels(ctx context.Context) ([]*voicebiopb.Model, error) {
	resp, err := c.voicebio.ListModels(ctx, &voicebiopb.ListModelsRequest{})
	if err != nil {
		return nil, err
	}

	return resp.Models, nil
}

func (c *Client) StreamingEnroll(ctx context.Context,
	cfg voicebiopb.EnrollmentConfig, //nolint:govet // cfg is a large struct but we want to use a copy
	audio io.Reader) (*voicebiopb.StreamingEnrollResponse, error) {

	stream, err := c.voicebio.StreamingEnroll(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := sendaudioEnroll(stream, &cfg, audio, c.streamingBufSize)
	if err != nil && err != io.EOF {
		// if sendaudio encountered io.EOF, it's only a
		// notification that the stream has closed.  The actual
		// status will be obtained in a subsequent Recv call, in
		// the other goroutine below.  We therefore only forward
		// non-EOF errors.
		return nil, err
	}

	return resp, nil
}

// sendaudio sends audio to a stream.
func sendaudioEnroll(stream voicebiopb.VoiceBioService_StreamingEnrollClient,
	cfg *voicebiopb.EnrollmentConfig, audio io.Reader,
	bufsize uint32) (*voicebiopb.StreamingEnrollResponse, error) {
	// The first message needs to be a config message, and all subsequent
	// messages must be audio messages.
	// Send the recognition config
	if err := stream.Send(&voicebiopb.StreamingEnrollRequest{
		Request: &voicebiopb.StreamingEnrollRequest_Config{Config: cfg},
	}); err != nil {
		// if this failed, we don't need to CloseSend
		return nil, err
	}

	// Stream the audio.
	buf := make([]byte, bufsize)

	for {
		n, err := audio.Read(buf)
		if n > 0 {
			if err2 := stream.Send(&voicebiopb.StreamingEnrollRequest{
				Request: &voicebiopb.StreamingEnrollRequest_Audio{
					Audio: &voicebiopb.Audio{Data: buf[:n]},
				},
			}); err2 != nil {
				// if we couldn't Send, the stream has
				// encountered an error and we don't need to
				// CloseSend.
				return nil, err2
			}
		}

		if err != nil {
			// err could be io.EOF, or some other error reading from
			// audio.  In any case, we need to CloseSend, send the
			// appropriate error to errch and return from the function
			if err == io.EOF {
				break
			} else if err2 := stream.CloseSend(); err2 != nil && err2 != io.EOF {
				return nil, err2
			} else if err != io.EOF {
				return nil, err
			}
		}
	}
	resp, err := stream.CloseAndRecv()
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *Client) StreamingVerify(ctx context.Context,
	cfg voicebiopb.VerificationConfig, //nolint:govet // cfg is a large struct but we want to use a copy
	audio io.Reader) (*voicebiopb.StreamingVerifyResponse, error) {

	stream, err := c.voicebio.StreamingVerify(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := sendaudioVerify(stream, &cfg, audio, c.streamingBufSize)
	if err != nil && err != io.EOF {
		// if sendaudio encountered io.EOF, it's only a
		// notification that the stream has closed.  The actual
		// status will be obtained in a subsequent Recv call, in
		// the other goroutine below.  We therefore only forward
		// non-EOF errors.
		return nil, err
	}

	return resp, nil
}

// sendaudio sends audio to a stream.
func sendaudioVerify(stream voicebiopb.VoiceBioService_StreamingVerifyClient,
	cfg *voicebiopb.VerificationConfig, audio io.Reader,
	bufsize uint32) (*voicebiopb.StreamingVerifyResponse, error) {
	// The first message needs to be a config message, and all subsequent
	// messages must be audio messages.
	// Send the recognition config
	if err := stream.Send(&voicebiopb.StreamingVerifyRequest{
		Request: &voicebiopb.StreamingVerifyRequest_Config{Config: cfg},
	}); err != nil {
		// if this failed, we don't need to CloseSend
		return nil, err
	}

	// Stream the audio.
	buf := make([]byte, bufsize)

	for {
		n, err := audio.Read(buf)
		if n > 0 {
			if err2 := stream.Send(&voicebiopb.StreamingVerifyRequest{
				Request: &voicebiopb.StreamingVerifyRequest_Audio{
					Audio: &voicebiopb.Audio{Data: buf[:n]},
				},
			}); err2 != nil {
				// if we couldn't Send, the stream has
				// encountered an error and we don't need to
				// CloseSend.
				break
			}
		}

		if err != nil {
			// err could be io.EOF, or some other error reading from
			// audio.  In any case, we need to CloseSend, send the
			// appropriate error to errch and return from the function
			if err == io.EOF {
				break
			} else if err2 := stream.CloseSend(); err2 != nil && err2 != io.EOF {
				return nil, err2
			} else if err != io.EOF {
				return nil, err
			}
		}
	}
	resp, err := stream.CloseAndRecv()
	if err != nil {
		return nil, err
	}

	return resp, err
}

func (c *Client) StreamingIdentify(ctx context.Context,
	cfg voicebiopb.IdentificationConfig, //nolint:govet // cfg is a large struct but we want to use a copy
	audio io.Reader) (*voicebiopb.StreamingIdentifyResponse, error) {

	stream, err := c.voicebio.StreamingIdentify(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := sendaudioIdentify(stream, &cfg, audio, c.streamingBufSize)
	if err != nil && err != io.EOF {
		// if sendaudio encountered io.EOF, it's only a
		// notification that the stream has closed.  The actual
		// status will be obtained in a subsequent Recv call, in
		// the other goroutine below.  We therefore only forward
		// non-EOF errors.
		return nil, err
	}

	return resp, nil
}

// sendaudio sends audio to a stream.
func sendaudioIdentify(stream voicebiopb.VoiceBioService_StreamingIdentifyClient,
	cfg *voicebiopb.IdentificationConfig, audio io.Reader,
	bufsize uint32) (*voicebiopb.StreamingIdentifyResponse, error) {
	// The first message needs to be a config message, and all subsequent
	// messages must be audio messages.
	// Send the recognition config
	if err := stream.Send(&voicebiopb.StreamingIdentifyRequest{
		Request: &voicebiopb.StreamingIdentifyRequest_Config{Config: cfg},
	}); err != nil {
		// if this failed, we don't need to CloseSend
		return nil, err
	}

	// Stream the audio.
	buf := make([]byte, bufsize)

	for {
		n, err := audio.Read(buf)
		if n > 0 {
			if err2 := stream.Send(&voicebiopb.StreamingIdentifyRequest{
				Request: &voicebiopb.StreamingIdentifyRequest_Audio{
					Audio: &voicebiopb.Audio{Data: buf[:n]},
				},
			}); err2 != nil {
				// if we couldn't Send, the stream has
				// encountered an error and we don't need to
				// CloseSend.
				return nil, err2
			}
		}

		if err != nil {
			// err could be io.EOF, or some other error reading from
			// audio.  In any case, we need to CloseSend, send the
			// appropriate error to errch and return from the function
			if err == io.EOF {
				break
			} else if err2 := stream.CloseSend(); err2 != nil && err2 != io.EOF {
				return nil, err2
			} else if err != io.EOF {
				return nil, err
			}
		}
	}
	resp, err := stream.CloseAndRecv()
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}
