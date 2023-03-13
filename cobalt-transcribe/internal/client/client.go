// Copyright (2020) Cobalt Speech and Language Inc.

// Package client wraps the sdk-cubic client and implements the interface
// tetracubic/client.Client that can use grpc to receive results from cubicsvr
// rather than directly from tetracubic.Tetracubic.
package client

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/cobaltspeech/log"

	"google.golang.org/grpc"

	cubicpb "github.com/cobaltspeech/go-genproto/cobaltspeech/cubic/v5"
)

const defaultStreamingBufsize uint32 = 1024

type Client struct {
	cubic            cubicpb.CubicServiceClient
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
		cubic:            cubicpb.NewCubicServiceClient(conn),
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
	v, err := c.cubic.Version(ctx, &cubicpb.VersionRequest{})
	if err != nil {
		return "", err
	}

	return v.Version, nil
}

// TODO (cenk) : implement the list models
func (c *Client) ListModels(ctx context.Context) ([]*cubicpb.Model, error) {
	resp, err := c.cubic.ListModels(ctx, &cubicpb.ListModelsRequest{})
	if err != nil {
		return nil, err
	}

	return resp.Models, nil
}

// RecognitionResponseHandler is a type of callback function that will be called
// when the `StreamingRecognize` method is running.  For each response received
// from cubic server, this method will be called once.  The provided
// RecognitionResponse is guaranteed to be non-nil.  Since this function is
// executed as part of the streaming process, it should preferably return
// quickly and certainly not block.
type RecognitionResponseHandler func(*cubicpb.StreamingRecognizeResponse)

func (c *Client) StreamingRecognize(ctx context.Context,
	cfg cubicpb.RecognitionConfig, //nolint:gocritic // cfg is a large struct but we want to use a copy
	audio io.Reader, handler RecognitionResponseHandler) error {

	var handlerErr error

	handlerpb := func(resp *cubicpb.StreamingRecognizeResponse) {
		if resp == nil {
			return
		}

		handler(resp)
	}

	stream, err := c.cubic.StreamingRecognize(ctx)
	if err != nil {
		return err
	}

	// There are two concurrent processes going on.  We will create a new
	// goroutine to read audio and stream it to the server.  This goroutine
	// will receive results from the stream.  Errors could occur in both
	// goroutines.  We therefore setup a channel, errch, to hold these
	// errors. Both goroutines are designed to send up to one error, and
	// return immediately. Therefore we use a bufferred channel with a
	// capacity of two.
	errch := make(chan error, 2) //nolint:gomnd // 2 is not magic number as explained above.

	// start streaming audio in a separate goroutine
	var wg sync.WaitGroup

	wg.Add(1)

	go func() {
		if err := sendaudio(stream, &cfg, audio, c.streamingBufSize); err != nil && err != io.EOF {
			// if sendaudio encountered io.EOF, it's only a
			// notification that the stream has closed.  The actual
			// status will be obtained in a subsequent Recv call, in
			// the other goroutine below.  We therefore only forward
			// non-EOF errors.
			errch <- err
		}

		wg.Done()
	}()

	for {
		in, err := stream.Recv()

		if err == io.EOF {
			break
		} else if err != nil {
			errch <- err
			break
		}

		handlerpb(in)
	}

	wg.Wait()

	select {
	case err := <-errch:
		// There may be more than one error in the channel, but it is
		// very likely they are related (e.g. connection reset causing
		// both the send and recv to fail) and we therefore return the
		// first error and discard the other.
		return fmt.Errorf("streaming recognition failed: %v", err)
	default:
	}

	if handlerErr != nil {
		return handlerErr
	}

	return nil
}

// sendaudio sends audio to a stream.
func sendaudio(stream cubicpb.CubicService_StreamingRecognizeClient,
	cfg *cubicpb.RecognitionConfig, audio io.Reader,
	bufsize uint32) error {
	// The first message needs to be a config message, and all subsequent
	// messages must be audio messages.
	// Send the recognition config
	if err := stream.Send(&cubicpb.StreamingRecognizeRequest{
		Request: &cubicpb.StreamingRecognizeRequest_Config{Config: cfg},
	}); err != nil {
		// if this failed, we don't need to CloseSend
		return err
	}

	// Stream the audio.
	buf := make([]byte, bufsize)

	for {
		n, err := audio.Read(buf)
		if n > 0 {
			if err2 := stream.Send(&cubicpb.StreamingRecognizeRequest{
				Request: &cubicpb.StreamingRecognizeRequest_Audio{
					Audio: &cubicpb.RecognitionAudio{Data: buf[:n]},
				},
			}); err2 != nil {
				// if we couldn't Send, the stream has
				// encountered an error and we don't need to
				// CloseSend.
				return err2
			}
		}

		if err != nil {
			// err could be io.EOF, or some other error reading from
			// audio.  In any case, we need to CloseSend, send the
			// appropriate error to errch and return from the function
			if err2 := stream.CloseSend(); err2 != nil {
				return err2
			} else if err != io.EOF {
				return err
			}

			return nil
		}
	}
}

// TODO (cenk) : implement compile context
func (c *Client) CompileContext(ctx context.Context,
	modelID, token string, phrases []cubicpb.ContextPhrase) (cubicpb.CompiledContext, error) {
	phraseList := make([]*cubicpb.ContextPhrase, len(phrases))

	for i, v := range phrases {
		phraseList[i] = &cubicpb.ContextPhrase{
			Text:  v.Text,
			Boost: v.Boost,
		}
	}

	compiled, err := c.cubic.CompileContext(ctx, &cubicpb.CompileContextRequest{
		ModelId: modelID,
		Token:   token,
		Phrases: phraseList,
	})
	if err != nil {
		return cubicpb.CompiledContext{}, err
	}

	return *compiled.Context, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}