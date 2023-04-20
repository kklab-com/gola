package gola

import (
	"context"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	buf "github.com/kklab-com/goth-bytebuf"
	erresponse "github.com/kklab-com/goth-erresponse"
	"github.com/stretchr/testify/assert"
)

type DefaultRootHandler struct {
}

func (d *DefaultRootHandler) Run(ctx context.Context, request Request, response Response) (er error) {
	response.SetHeader("ROOT", "VAL")
	return nil
}

type DefaultEmptyHandler struct {
}

func (d *DefaultEmptyHandler) Run(ctx context.Context, request Request, response Response) (er error) {
	response.SetContentType("text/plain")
	return nil
}

type DefaultJSONHandler struct {
}

func (d *DefaultJSONHandler) Run(ctx context.Context, request Request, response Response) (er error) {
	response.JSONResponse(buf.EmptyByteBuf().WriteString("{}"))
	return nil
}

type DefaultWildHandler struct {
}

func (d *DefaultWildHandler) Run(ctx context.Context, request Request, response Response) (er error) {
	response.JSONResponse(buf.EmptyByteBuf().WriteString("{\"type\":1}"))
	return nil
}

type DefaultBadHandler struct {
}

func (d *DefaultBadHandler) Run(ctx context.Context, request Request, response Response) (er error) {
	return erresponse.ServerErrorCacheOperationFail
}

func TestRoute_SetEndpoint(t *testing.T) {
	goLA := NewServe()
	route := goLA.Route()
	route.SetRootHandlers(&DefaultRootHandler{})
	route.
		SetEndpoint("/auth/group/user/:user_id", &DefaultEmptyHandler{}).
		SetEndpoint("/auth/group/user/:user_id/book/:book", &DefaultEmptyHandler{}, &DefaultJSONHandler{}).
		SetEndpoint("/auth/group/user/:user_id/profile", &DefaultEmptyHandler{}).
		SetEndpoint("/bad", &DefaultBadHandler{}).
		SetEndpoint("/wild/*", &DefaultWildHandler{})

	node, parameters, _ := route.RouteNode("/auth/group/user/123")
	assert.NotNil(t, node)
	assert.Equal(t, 1, len(node.Handlers()))
	assert.Equal(t, "123", parameters["user_id"].String())

	node, parameters, _ = route.RouteNode("/auth/group/user/123/book")
	assert.NotNil(t, node)
	assert.Equal(t, 2, len(node.Handlers()))
	assert.Nil(t, parameters["book"])

	node, parameters, _ = route.RouteNode("/auth/group/user/123/book/newbook")
	assert.NotNil(t, node)
	assert.Equal(t, 2, len(node.Handlers()))
	assert.Equal(t, "newbook", parameters["book"].String())

	node, parameters, _ = route.RouteNode("/auth/group/user")
	assert.NotNil(t, node)
	assert.Equal(t, 1, len(node.Handlers()))
	assert.Nil(t, parameters["user"])
	assert.Nil(t, parameters["book"])

	node, parameters, _ = route.RouteNode("/auth/group/user/123/profile/info/myname")
	assert.Nil(t, node)
	assert.Nil(t, parameters["user"])
	assert.Nil(t, parameters["book"])

	node, parameters, _ = route.RouteNode("/auth/group/user/123/book/newbook/dasdqwe")
	assert.Nil(t, node)
	assert.Nil(t, parameters["user"])
	assert.Nil(t, parameters["book"])

	node, parameters, _ = route.RouteNode("/wild/card/new")
	assert.NotNil(t, node)
	assert.Nil(t, parameters["user"])
	assert.Nil(t, parameters["book"])
	assert.NotNil(t, parameters["wild"])
	assert.Equal(t, 1, len(parameters))
	assert.Equal(t, NodeTypeRecursive, node.NodeType())

	response, err := goLA.Register(nil, events.ALBTargetGroupRequest{Path: "/auth/group/user/123"})
	assert.NotNil(t, response)
	assert.Equal(t, 200, response.StatusCode)
	assert.Nil(t, err)

	response, err = goLA.Register(nil, events.ALBTargetGroupRequest{Path: "/auth/group/user/123/goodgame"})
	assert.NotNil(t, response)
	assert.Equal(t, 404, response.StatusCode)
	assert.Nil(t, err)

	response, err = goLA.Register(nil, events.ALBTargetGroupRequest{Path: "/bad"})
	assert.NotNil(t, response)
	assert.Equal(t, 500, response.StatusCode)
	assert.Nil(t, err)
}
