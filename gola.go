package gola

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"

	"github.com/aws/aws-lambda-go/events"
	httpheadername "github.com/kklab-com/gone-httpheadername"
	buf "github.com/kklab-com/goth-bytebuf"
	erresponse "github.com/kklab-com/goth-erresponse"
	kkpanic "github.com/kklab-com/goth-panic"
)

type GoLA struct {
	route                               *Route
	NotFoundHandler, ServerErrorHandler Handler
}

func NewServe() *GoLA {
	return &GoLA{
		route:              NewRoute(),
		NotFoundHandler:    &DefaultNotFoundHandler{},
		ServerErrorHandler: &DefaultServerErrorHandler{},
	}
}

func (g *GoLA) Route() *Route {
	return g.route
}

var NotImplemented = erresponse.NotImplemented

func (g *GoLA) Register(ctx context.Context, request events.ALBTargetGroupRequest) (events.ALBTargetGroupResponse, error) {
	ctx = context.WithValue(ctx, "gola", g)
	node, parameters, isLast := g.route.RouteNode(request.Path)
	req := newRequest(request, parameters)
	resp := newResponse()
	var lErr error
	if node == nil {
		lErr = g.NotFoundHandler.Run(ctx, req, resp)
	} else {
		ctx = context.WithValue(ctx, "gola-node", node)
		stopSig := &atomic.Bool{}
		for _, handler := range node.Handlers() {
			if stopSig.Load() {
				break
			}

			lErr = func(handler Handler, ctx context.Context, request Request, response Response, isLast bool) error {
				var err error
				stop := false
				if httpHandler, ok := handler.(HttpHandler); ok {
					defer func(handler HttpHandler, ctx context.Context, request Request, response Response) {
						if r := recover(); r != nil {
							erErr := &ErrorResponseImpl{
								ErrorResponse: erresponse.ServerErrorPanic,
							}

							switch er := r.(type) {
							case *kkpanic.CaughtImpl:
								erErr.Caught = er
							default:
								erErr.Caught = kkpanic.Convert(er)
							}

							handler.ErrorCaught(ctx, req, resp, erErr)
							wrapErrorResponse(erErr, resp)
						}

					}(httpHandler, ctx, req, resp)

					if err, stop = g.funcExecutor(httpHandler.Before, ctx, req, resp); err != nil {
						stopSig.Store(stop)
						return err
					}

					switch {
					case request.Method() == http.MethodGet:
						if isLast {
							if err, stop = g.funcExecutor(httpHandler.Index, ctx, request, response); err == nil {
								break
							} else if err != NotImplemented {
								stopSig.Store(stop)
								return err
							}
						}

						err, stop = g.funcExecutor(httpHandler.Get, ctx, request, response)
					case request.Method() == http.MethodPost:
						err, stop = g.funcExecutor(httpHandler.Post, ctx, request, response)
					case request.Method() == http.MethodPut:
						err, stop = g.funcExecutor(httpHandler.Put, ctx, request, response)
					case request.Method() == http.MethodDelete:
						err, stop = g.funcExecutor(httpHandler.Delete, ctx, request, response)
					case request.Method() == http.MethodOptions:
						err, stop = g.funcExecutor(httpHandler.Options, ctx, request, response)
					case request.Method() == http.MethodPatch:
						err, stop = g.funcExecutor(httpHandler.Patch, ctx, request, response)
					case request.Method() == http.MethodTrace:
						err, stop = g.funcExecutor(httpHandler.Trace, ctx, request, response)
					case request.Method() == http.MethodConnect:
						err, stop = g.funcExecutor(httpHandler.Connect, ctx, request, response)
					default:
						err, stop = erresponse.MethodNotAllowed, true
					}

					if err != nil || stop {
						stopSig.Store(stop)
						return err
					}

					if err, stop = g.funcExecutor(httpHandler.After, ctx, req, resp); err != nil {
						stopSig.Store(stop)
						return err
					}
				} else {
					err, stop = g.funcExecutor(handler.Run, ctx, req, resp)
				}

				stopSig.Store(stop)
				return err
			}(handler, ctx, req, resp, isLast)
		}
	}

	return *resp.Build(), lErr
}

func (g *GoLA) funcExecutor(f func(ctx context.Context, request Request, response Response) error, ctx context.Context, request Request, response Response) (er error, stop bool) {
	if err := f(ctx, request, response); err != nil {
		if err == NotImplemented {
			return err, false
		}

		ctx = context.WithValue(ctx, "gola-handler-error", err)
		if response.StatusCode() != 0 {
			return err, true
		} else {
			if v, ok := err.(erresponse.ErrorResponse); ok {
				wrapErrorResponse(v, response)
				return nil, true
			} else {
				return g.ServerErrorHandler.Run(ctx, request, response), true
			}
		}
	}

	return nil, false
}

func wrapErrorResponse(err erresponse.ErrorResponse, resp Response) {
	resp.
		SetStatusCode(err.ErrorStatusCode()).
		JSONResponse(buf.NewByteBufString(err.Error()))
}

func CORSHelper(request Request, response Response) {
	headers := map[string]string{}
	if v := request.GetHeader(httpheadername.Origin); v == "null" {
		response.AddHeader(httpheadername.AccessControlAllowOrigin, "*")
	} else {
		response.AddHeader(httpheadername.AccessControlAllowOrigin, v)
	}

	if str := request.GetHeader(httpheadername.AccessControlRequestHeaders); str != "" {
		response.AddHeader(httpheadername.AccessControlAllowHeaders, str)
	}

	if str := request.GetHeader(httpheadername.AccessControlRequestMethod); str != "" {
		response.AddHeader(httpheadername.AccessControlAllowMethods, str)
		headers["access-control-allow-methods"] = str
	}
}

type ErrorResponseImpl struct {
	erresponse.ErrorResponse
	Caught *kkpanic.CaughtImpl `json:"caught,omitempty"`
}

func (e *ErrorResponseImpl) String() string {
	return e.Caught.String()
}

type Handler interface {
	Run(ctx context.Context, request Request, response Response) (er error)
}

type DefaultHandler struct {
}

func (d *DefaultHandler) Run(ctx context.Context, request Request, response Response) (er error) {
	CORSHelper(request, response)
	response.SetContentType("text/plain")
	return nil
}

type HttpHandler interface {
	Index(ctx context.Context, request Request, response Response) (er error)
	Get(ctx context.Context, request Request, response Response) (er error)
	Post(ctx context.Context, request Request, response Response) (er error)
	Put(ctx context.Context, request Request, response Response) (er error)
	Delete(ctx context.Context, request Request, response Response) (er error)
	Options(ctx context.Context, request Request, response Response) (er error)
	Patch(ctx context.Context, request Request, response Response) (er error)
	Trace(ctx context.Context, request Request, response Response) (er error)
	Connect(ctx context.Context, request Request, response Response) (er error)
	Before(ctx context.Context, request Request, response Response) (er error)
	After(ctx context.Context, request Request, response Response) (er error)
	ErrorCaught(ctx context.Context, request Request, response Response, err erresponse.ErrorResponse)
}

type DefaultHttpHandler struct {
	DefaultHandler
}

func (h *DefaultHttpHandler) Index(ctx context.Context, request Request, response Response) (er error) {
	return NotImplemented
}

func (h *DefaultHttpHandler) Get(ctx context.Context, request Request, response Response) (er error) {
	return nil
}

func (h *DefaultHttpHandler) Post(ctx context.Context, request Request, response Response) (er error) {
	return nil
}

func (h *DefaultHttpHandler) Put(ctx context.Context, request Request, response Response) (er error) {
	return nil
}

func (h *DefaultHttpHandler) Delete(ctx context.Context, request Request, response Response) (er error) {
	return nil
}

func (h *DefaultHttpHandler) Options(ctx context.Context, request Request, response Response) (er error) {
	return nil
}

func (h *DefaultHttpHandler) Patch(ctx context.Context, request Request, response Response) (er error) {
	return nil
}

func (h *DefaultHttpHandler) Trace(ctx context.Context, request Request, response Response) (er error) {
	return nil
}

func (h *DefaultHttpHandler) Connect(ctx context.Context, request Request, response Response) (er error) {
	return nil
}

func (h *DefaultHttpHandler) Before(ctx context.Context, request Request, response Response) (er error) {
	return nil
}

func (h *DefaultHttpHandler) After(ctx context.Context, request Request, response Response) (er error) {
	return nil
}

func (h *DefaultHttpHandler) ErrorCaught(ctx context.Context, request Request, response Response, err erresponse.ErrorResponse) {
	println(err.(fmt.Stringer).String())
}

type DefaultNotFoundHandler struct {
}

func (d *DefaultNotFoundHandler) Run(ctx context.Context, request Request, response Response) (er error) {
	CORSHelper(request, response)
	response.
		SetStatusCode(erresponse.NotFound.ErrorStatusCode()).
		JSONResponse(buf.NewByteBufString(erresponse.NotFound.Error()))

	return nil
}

type DefaultServerErrorHandler struct {
}

func (d *DefaultServerErrorHandler) Run(ctx context.Context, request Request, response Response) (er error) {
	CORSHelper(request, response)
	response.
		SetStatusCode(erresponse.ServerError.ErrorStatusCode()).
		JSONResponse(buf.NewByteBufString(erresponse.ServerError.Error()))

	return nil
}

type DefaultCORSHandler struct {
}

func (d *DefaultCORSHandler) Run(ctx context.Context, request Request, response Response) (er error) {
	CORSHelper(request, response)
	return nil
}
