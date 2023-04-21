package gola

import (
	"context"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	httpheadername "github.com/kklab-com/gone-httpheadername"
	buf "github.com/kklab-com/goth-bytebuf"
	erresponse "github.com/kklab-com/goth-erresponse"
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
		for _, handler := range node.Handlers() {
			lErr = func(handler Handler, ctx context.Context, request Request, response Response, isLast bool) error {
				var err error
				if httpHandler, ok := handler.(HttpHandler); ok {
					if err = g.funcExecutor(httpHandler.Before, ctx, req, resp); err != nil {
						return err
					}

					switch {
					case request.Method() == http.MethodGet:
						if isLast {
							if err = g.funcExecutor(httpHandler.Index, ctx, request, response); err == nil {
								break
							} else if err != NotImplemented {
								return err
							}
						}

						err = g.funcExecutor(httpHandler.Get, ctx, request, response)
					case request.Method() == http.MethodPost:
						err = g.funcExecutor(httpHandler.Post, ctx, request, response)
					case request.Method() == http.MethodPut:
						err = g.funcExecutor(httpHandler.Put, ctx, request, response)
					case request.Method() == http.MethodDelete:
						err = g.funcExecutor(httpHandler.Delete, ctx, request, response)
					case request.Method() == http.MethodOptions:
						err = g.funcExecutor(httpHandler.Options, ctx, request, response)
					case request.Method() == http.MethodPatch:
						err = g.funcExecutor(httpHandler.Patch, ctx, request, response)
					case request.Method() == http.MethodTrace:
						err = g.funcExecutor(httpHandler.Trace, ctx, request, response)
					case request.Method() == http.MethodConnect:
						err = g.funcExecutor(httpHandler.Connect, ctx, request, response)
					}

					if err = g.funcExecutor(httpHandler.After, ctx, req, resp); err != nil {
						return err
					}
				} else {
					err = g.funcExecutor(handler.Run, ctx, req, resp)
				}

				return err
			}(handler, ctx, req, resp, isLast)
		}
	}

	return *resp.Build(), lErr
}

func (g *GoLA) funcExecutor(f func(ctx context.Context, request Request, response Response) error, ctx context.Context, request Request, response Response) error {
	if err := f(ctx, request, response); err != nil {
		if err == NotImplemented {
			return err
		}

		ctx = context.WithValue(ctx, "gola-handler-error", err)
		if response.StatusCode() != 0 {
			return err
		} else {
			if v, ok := err.(erresponse.ErrorResponse); ok {
				wrapErrorResponse(v, response)
				return nil
			} else {
				return g.ServerErrorHandler.Run(ctx, request, response)
			}
		}
	}

	return nil
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
