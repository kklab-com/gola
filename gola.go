package gola

import (
	"context"
	"fmt"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	httpheadername "github.com/kklab-com/gone-httpheadername"
	buf "github.com/kklab-com/goth-bytebuf"
	erresponse "github.com/kklab-com/goth-erresponse"
	kkpanic "github.com/kklab-com/goth-panic"
)

type GoLA struct {
	route                                                            *Route
	ctxInjectMap                                                     map[any]any
	BeginHandler, NotFoundHandler, ServerErrorHandler, FinishHandler Handler
}

func NewServe() *GoLA {
	return &GoLA{
		route:              NewRoute(),
		ctxInjectMap:       map[any]any{},
		BeginHandler:       &DefaultEmptyHandler{},
		NotFoundHandler:    &DefaultNotFoundHandler{},
		ServerErrorHandler: &DefaultServerErrorHandler{},
		FinishHandler:      &DefaultEmptyHandler{},
	}
}

func (g *GoLA) Route() *Route {
	return g.route
}

func (g *GoLA) ContextInject(key any, value any) *GoLA {
	g.ctxInjectMap[key] = value
	return g
}

func (g *GoLA) Context(key any) any {
	if v, f := g.ctxInjectMap[key]; f {
		return v
	}

	return nil
}

var NotImplemented = erresponse.NotImplemented

const (
	CtxGoLA             = "gola"
	CtxGoLAParams       = "gola-params"
	CtxGoLANode         = "gola-node"
	CtxGoLANodeLast     = "gola-node-last"
	CtxGoLAHandler      = "gola-handler"
	CtxGoLAHandlerError = "gola-handler-error"
)

func (g *GoLA) Register(ctx context.Context, request events.ALBTargetGroupRequest) (events.ALBTargetGroupResponse, error) {
	ctx = context.WithValue(ctx, CtxGoLA, g)
	ctx = context.WithValue(ctx, CtxGoLAParams, map[string]any{})
	node, parameters, isLast := g.route.RouteNode(request.Path)
	req := NewRequest(request, parameters)
	resp := NewResponse()
	for k, v := range g.ctxInjectMap {
		ctx = context.WithValue(ctx, k, v)
	}

	if err := g.BeginHandler.Run(ctx, req, resp); err != nil {
		panic(err)
	}

	var lErr error
	if node == nil {
		lErr = g.NotFoundHandler.Run(ctx, req, resp)
	} else {
		ctx = context.WithValue(ctx, CtxGoLANode, node)
		ctx = context.WithValue(ctx, CtxGoLANodeLast, isLast)
		for _, handler := range node.Handlers() {
			ctx = context.WithValue(ctx, CtxGoLAHandler, handler)
			if err := handler.Run(ctx, req, resp); err != nil {
				ctx = context.WithValue(ctx, CtxGoLAHandlerError, err)
				if resp.StatusCode() != 0 {
					lErr = err
				} else {
					if v, ok := err.(erresponse.ErrorResponse); ok {
						wrapErrorResponse(v, resp)
					} else {
						lErr = g.ServerErrorHandler.Run(ctx, req, resp)
					}
				}

				break
			}
		}
	}

	if err := g.FinishHandler.Run(ctx, req, resp); err != nil {
		panic(err)
	}

	return *resp.Build(), lErr
}

func wrapErrorResponse(err erresponse.ErrorResponse, resp Response) {
	resp.
		SetStatusCode(err.ErrorStatusCode()).
		JSONResponse(buf.NewByteBufString(err.Error()))
}

func CORSHelper(request Request, response Response) {
	if v := request.GetHeader(httpheadername.Origin); v == "null" {
		response.SetHeader(httpheadername.AccessControlAllowOrigin, "*")
	} else {
		response.SetHeader(httpheadername.AccessControlAllowOrigin, v)
	}

	if str := request.GetHeader(httpheadername.AccessControlRequestHeaders); str != "" {
		response.SetHeader(httpheadername.AccessControlAllowHeaders, str)
	}

	if str := request.GetHeader(httpheadername.AccessControlRequestMethod); str != "" {
		response.SetHeader(httpheadername.AccessControlAllowMethods, str)
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

func (d *DefaultHandler) GoLA(ctx context.Context) *GoLA {
	return ctx.Value(CtxGoLA).(*GoLA)
}

func (d *DefaultHandler) Node(ctx context.Context) Node {
	return ctx.Value(CtxGoLANode).(Node)
}

func (d *DefaultHandler) IsLastNode(ctx context.Context) bool {
	return ctx.Value(CtxGoLANodeLast).(bool)
}

func (d *DefaultHandler) GetParam(ctx context.Context, key string) any {
	return ctx.Value(CtxGoLAParams).(map[string]any)[key]
}

func (d *DefaultHandler) SetParam(ctx context.Context, key string, value any) {
	ctx.Value(CtxGoLAParams).(map[string]any)[key] = value
}

func (d *DefaultHandler) Run(ctx context.Context, request Request, response Response) (er error) {
	CORSHelper(request, response)
	response.SetContentType("text/plain")
	return nil
}

type HttpHandler interface {
	Index(ctx context.Context, request Request, response Response) (er error)
	Get(ctx context.Context, request Request, response Response) (er error)
	Create(ctx context.Context, request Request, response Response) (er error)
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

func (h *DefaultHttpHandler) Run(ctx context.Context, request Request, response Response) (er error) {
	handler := ctx.Value(CtxGoLAHandler).(Handler)
	httpHandler, ok := handler.(HttpHandler)
	var err error
	if !ok {
		return
	}

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

			ctx = context.WithValue(ctx, CtxGoLAHandlerError, erErr)
			wrapErrorResponse(erErr, response)
			handler.ErrorCaught(ctx, request, response, erErr)
		}

	}(httpHandler, ctx, request, response)

	if err = httpHandler.Before(ctx, request, response); err != nil {
		return err
	}

	switch {
	case request.Method() == http.MethodGet:
		if ctx.Value(CtxGoLANodeLast).(bool) {
			if err = httpHandler.Index(ctx, request, response); err == nil {
				break
			} else if err != NotImplemented {
				return err
			}
		}

		err = httpHandler.Get(ctx, request, response)
	case request.Method() == http.MethodPost:
		if ctx.Value(CtxGoLANodeLast).(bool) {
			if err = httpHandler.Create(ctx, request, response); err == nil {
				break
			} else if err != NotImplemented {
				return err
			}
		}

		err = httpHandler.Post(ctx, request, response)
	case request.Method() == http.MethodPut:
		err = httpHandler.Put(ctx, request, response)
	case request.Method() == http.MethodDelete:
		err = httpHandler.Delete(ctx, request, response)
	case request.Method() == http.MethodOptions:
		err = httpHandler.Options(ctx, request, response)
	case request.Method() == http.MethodPatch:
		err = httpHandler.Patch(ctx, request, response)
	case request.Method() == http.MethodTrace:
		err = httpHandler.Trace(ctx, request, response)
	case request.Method() == http.MethodConnect:
		err = httpHandler.Connect(ctx, request, response)
	default:
		err = erresponse.MethodNotAllowed
	}

	if err != nil {
		return err
	}

	if err = httpHandler.After(ctx, request, response); err != nil {
		return err
	}

	return nil
}

func (h *DefaultHttpHandler) Index(ctx context.Context, request Request, response Response) (er error) {
	return NotImplemented
}

func (h *DefaultHttpHandler) Get(ctx context.Context, request Request, response Response) (er error) {
	return nil
}

func (h *DefaultHttpHandler) Create(ctx context.Context, request Request, response Response) (er error) {
	return NotImplemented
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

type DefaultEmptyHandler struct {
	DefaultHandler
}

func (d *DefaultEmptyHandler) Run(ctx context.Context, request Request, response Response) (er error) {
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
