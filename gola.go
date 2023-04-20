package gola

import (
	"context"

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

func (g *GoLA) Register(ctx context.Context, request events.ALBTargetGroupRequest) (events.ALBTargetGroupResponse, error) {
	node, parameters, _ := g.route.RouteNode(request.Path)
	req := newRequest(request, parameters)
	resp := newResponse()
	if node == nil {
		err := g.NotFoundHandler.Run(ctx, req, resp)
		return *resp.Build(), err
	} else {
		for _, handler := range node.Handlers() {
			if err := handler.Run(ctx, req, resp); err != nil {
				if resp.StatusCode() == 0 {
					if v, ok := err.(erresponse.ErrorResponse); ok {
						wrapErrorResponse(v, resp)
					} else {
						err = g.ServerErrorHandler.Run(ctx, req, resp)
					}
				}

				return *resp.Build(), nil
			}
		}

		return *resp.Build(), nil
	}

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

	return nil
}
