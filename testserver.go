package connectrpc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	"connectrpc.com/connect"
	pingv1 "github.com/bumberboy/xk6-connectrpc/testdata/ping/v1"
	"github.com/bumberboy/xk6-connectrpc/testdata/ping/v1/pingv1connect"
)

const (
	clientHeader        = "client-header"
	handlerHeader       = "handler-header"
	handlerTrailer      = "handler-trailer"
	headerValue         = "some-value"
	trailerValue        = "some-trailer-value"
	errorMessage        = "oh no"
	middlewareErrorCode = connect.CodeInvalidArgument
)

// pingServer implements the PingService for testing
type pingServer struct {
	pingv1connect.UnimplementedPingServiceHandler

	checkMetadata       bool
	includeErrorDetails bool
}

func (p pingServer) Ping(ctx context.Context, request *connect.Request[pingv1.PingRequest]) (*connect.Response[pingv1.PingResponse], error) {
	if err := expectClientHeader(p.checkMetadata, request); err != nil {
		return nil, err
	}
	if request.Peer().Addr == "" {
		return nil, connect.NewError(connect.CodeInternal, errors.New("no peer address"))
	}
	if request.Peer().Protocol == "" {
		return nil, connect.NewError(connect.CodeInternal, errors.New("no peer protocol"))
	}

	response := connect.NewResponse(
		&pingv1.PingResponse{
			Number: request.Msg.GetNumber(),
			Text:   request.Msg.GetText(),
		},
	)
	response.Header().Set(handlerHeader, headerValue)
	response.Trailer().Set(handlerTrailer, trailerValue)
	return response, nil
}

func (p pingServer) Fail(ctx context.Context, request *connect.Request[pingv1.FailRequest]) (*connect.Response[pingv1.FailResponse], error) {
	if err := expectClientHeader(p.checkMetadata, request); err != nil {
		return nil, err
	}
	if request.Peer().Addr == "" {
		return nil, connect.NewError(connect.CodeInternal, errors.New("no peer address"))
	}
	if request.Peer().Protocol == "" {
		return nil, connect.NewError(connect.CodeInternal, errors.New("no peer protocol"))
	}

	err := connect.NewError(connect.Code(request.Msg.GetCode()), errors.New(errorMessage))
	err.Meta().Set(handlerHeader, headerValue)
	err.Meta().Set(handlerTrailer, trailerValue)
	if p.includeErrorDetails {
		detail, derr := connect.NewErrorDetail(&pingv1.FailRequest{Code: request.Msg.GetCode()})
		if derr != nil {
			return nil, derr
		}
		err.AddDetail(detail)
	}
	return nil, err
}

func (p pingServer) Sum(
	ctx context.Context,
	stream *connect.ClientStream[pingv1.SumRequest],
) (*connect.Response[pingv1.SumResponse], error) {
	if p.checkMetadata {
		if err := expectMetadata(stream.RequestHeader(), "header", clientHeader, headerValue); err != nil {
			return nil, err
		}
	}
	if stream.Peer().Addr == "" {
		return nil, connect.NewError(connect.CodeInternal, errors.New("no peer address"))
	}
	if stream.Peer().Protocol == "" {
		return nil, connect.NewError(connect.CodeInternal, errors.New("no peer protocol"))
	}

	var sum int64
	for stream.Receive() {
		sum += stream.Msg().GetNumber()
	}
	if stream.Err() != nil {
		return nil, stream.Err()
	}

	response := connect.NewResponse(&pingv1.SumResponse{Sum: sum})
	response.Header().Set(handlerHeader, headerValue)
	response.Trailer().Set(handlerTrailer, trailerValue)
	return response, nil
}

func (p pingServer) CountUp(
	ctx context.Context,
	request *connect.Request[pingv1.CountUpRequest],
	stream *connect.ServerStream[pingv1.CountUpResponse],
) error {
	if err := expectClientHeader(p.checkMetadata, request); err != nil {
		return err
	}
	if request.Peer().Addr == "" {
		return connect.NewError(connect.CodeInternal, errors.New("no peer address"))
	}
	if request.Peer().Protocol == "" {
		return connect.NewError(connect.CodeInternal, errors.New("no peer protocol"))
	}
	if request.Msg.GetNumber() <= 0 {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf(
			"number must be positive: got %v",
			request.Msg.GetNumber(),
		))
	}

	stream.ResponseHeader().Set(handlerHeader, headerValue)
	stream.ResponseTrailer().Set(handlerTrailer, trailerValue)
	for i := int64(1); i <= request.Msg.GetNumber(); i++ {
		if err := stream.Send(&pingv1.CountUpResponse{Number: i}); err != nil {
			return err
		}
	}
	return nil
}

func (p pingServer) CumSum(
	ctx context.Context,
	stream *connect.BidiStream[pingv1.CumSumRequest, pingv1.CumSumResponse],
) error {
	var sum int64
	if p.checkMetadata {
		if err := expectMetadata(stream.RequestHeader(), "header", clientHeader, headerValue); err != nil {
			return err
		}
	}
	if stream.Peer().Addr == "" {
		return connect.NewError(connect.CodeInternal, errors.New("no peer address"))
	}
	if stream.Peer().Protocol == "" {
		return connect.NewError(connect.CodeInternal, errors.New("no peer protocol"))
	}

	stream.ResponseHeader().Set(handlerHeader, headerValue)
	stream.ResponseTrailer().Set(handlerTrailer, trailerValue)
	for {
		msg, err := stream.Receive()
		if errors.Is(err, io.EOF) {
			return nil
		} else if err != nil {
			return err
		}
		sum += msg.GetNumber()
		if err := stream.Send(&pingv1.CumSumResponse{Sum: sum}); err != nil {
			return err
		}
	}
}

// Helper functions for metadata validation
func expectClientHeader(checkMetadata bool, request interface{ Header() http.Header }) error {
	if !checkMetadata {
		return nil
	}
	return expectMetadata(request.Header(), "header", clientHeader, headerValue)
}

func expectMetadata(headers http.Header, typ, key, expectedValue string) error {
	if value := headers.Get(key); value != expectedValue {
		return connect.NewError(
			middlewareErrorCode,
			fmt.Errorf(
				"expected %s %q: got %q",
				typ,
				expectedValue,
				value,
			),
		)
	}
	return nil
}

// Test server factory functions
func newTestServer(checkMetadata bool) *httptest.Server {
	server := pingServer{
		checkMetadata:       checkMetadata,
		includeErrorDetails: false,
	}

	mux := http.NewServeMux()
	path, handler := pingv1connect.NewPingServiceHandler(server)
	mux.Handle(path, handler)

	return httptest.NewServer(mux)
}

func newTLSTestServer(checkMetadata bool) *httptest.Server {
	server := pingServer{
		checkMetadata:       checkMetadata,
		includeErrorDetails: false,
	}

	mux := http.NewServeMux()
	path, handler := pingv1connect.NewPingServiceHandler(server)
	mux.Handle(path, handler)

	return httptest.NewTLSServer(mux)
}

// Exported functions for testing
func NewTestServer(checkMetadata bool) *httptest.Server {
	return newTestServer(checkMetadata)
}

func NewTLSTestServer(checkMetadata bool) *httptest.Server {
	return newTLSTestServer(checkMetadata)
}

// NewTestServerWithErrorDetails creates a test server with error details enabled for testing
func NewTestServerWithErrorDetails(checkMetadata bool) *httptest.Server {
	server := pingServer{
		checkMetadata:       checkMetadata,
		includeErrorDetails: true,
	}

	mux := http.NewServeMux()
	path, handler := pingv1connect.NewPingServiceHandler(server)
	mux.Handle(path, handler)

	return httptest.NewServer(mux)
}
