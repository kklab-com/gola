package gola

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	buf "github.com/kklab-com/goth-bytebuf"
	erresponse "github.com/kklab-com/goth-erresponse"
	"github.com/stretchr/testify/assert"
)

type GOLATestRegisterHandler struct {
	DefaultHttpHandler
}

func (h *GOLATestRegisterHandler) Index(ctx context.Context, request Request, response Response) (er error) {
	t := ctx.Value("t").(*testing.T)
	assert.NotNil(t, h.GoLA(ctx))
	assert.NotNil(t, h.Node(ctx))
	assert.True(t, h.IsLastNode(ctx))
	assert.True(t, ctx.Value(CtxGoLANodeLast).(bool))
	response.SetBody(buf.NewByteBufString("INDEX"))
	return
}

func (h *GOLATestRegisterHandler) Options(ctx context.Context, request Request, response Response) (er error) {
	response.SetBody(buf.NewByteBufString("OPTIONS"))
	return
}

func (h *GOLATestRegisterHandler) Get(ctx context.Context, request Request, response Response) (er error) {
	assert.False(ctx.Value("t").(*testing.T), ctx.Value(CtxGoLANodeLast).(bool))
	response.SetBody(buf.NewByteBufString("GET"))
	return
}

type GOLATestRegisterNoIndexHandler struct {
	DefaultHttpHandler
}

func (h *GOLATestRegisterNoIndexHandler) Get(ctx context.Context, request Request, response Response) (er error) {
	response.SetBody(buf.NewByteBufString("GET"))
	return
}

type GOLATestRegisterBadHandler struct {
	DefaultHttpHandler
}

func (h *GOLATestRegisterBadHandler) Get(ctx context.Context, request Request, response Response) (er error) {
	return erresponse.ServerError
}

type GOLATestRegisterBadAfterHandler struct {
	DefaultHttpHandler
	t *testing.T
}

func (h *GOLATestRegisterBadAfterHandler) Get(ctx context.Context, request Request, response Response) (er error) {
	h.t.Fail()
	return nil
}

type GOLATestRegisterPanicHandler struct {
	DefaultHttpHandler
	t *testing.T
}

func (h *GOLATestRegisterPanicHandler) Get(ctx context.Context, request Request, response Response) (er error) {
	panic("panic")
}

func (h *GOLATestRegisterPanicHandler) ErrorCaught(ctx context.Context, request Request, response Response, err erresponse.ErrorResponse) {
	assert.Equal(h.t, 500, err.ErrorStatusCode())
}

func TestGoLA_Register(t *testing.T) {
	goLA := NewServe()
	goLA.ContextInject("t", t)
	route := goLA.Route()
	route.SetEndpoint("/auth/group/user/:user_id", &GOLATestRegisterHandler{})
	route.SetEndpoint("/auth/group/noIndex/:noIndex", &GOLATestRegisterNoIndexHandler{})
	route.SetEndpoint("/auth/group/bad", &GOLATestRegisterBadHandler{}, &GOLATestRegisterBadAfterHandler{t: t})
	route.SetEndpoint("/auth/group/panic", &GOLATestRegisterPanicHandler{t: t})
	route.SetEndpoint("/auth/abc/cde", &GOLATestRegisterHandler{})
	response, err := goLA.Register(context.Background(), events.ALBTargetGroupRequest{Path: "/auth/group/user/123", HTTPMethod: "OPTIONS", MultiValueHeaders: map[string][]string{"access-control-request-headers": {"content-type"}, "access-control-request-method": {"POST"}}})
	assert.Equal(t, 200, response.StatusCode)
	assert.Equal(t, base64.StdEncoding.EncodeToString([]byte("OPTIONS")), response.Body)
	assert.Nil(t, err)

	response, err = goLA.Register(context.Background(), events.ALBTargetGroupRequest{Path: "/auth/group/user/123", HTTPMethod: "GET", MultiValueHeaders: map[string][]string{"access-control-request-headers": {"content-type"}, "access-control-request-method": {"POST"}}})
	assert.Equal(t, 200, response.StatusCode)
	assert.Equal(t, base64.StdEncoding.EncodeToString([]byte("GET")), response.Body)
	assert.Nil(t, err)

	response, err = goLA.Register(context.Background(), events.ALBTargetGroupRequest{Path: "/auth/group/user/", HTTPMethod: "GET", MultiValueHeaders: map[string][]string{"access-control-request-headers": {"content-type"}, "access-control-request-method": {"POST"}}})
	assert.Equal(t, 200, response.StatusCode)
	assert.Equal(t, base64.StdEncoding.EncodeToString([]byte("INDEX")), response.Body)
	assert.Nil(t, err)

	response, err = goLA.Register(context.Background(), events.ALBTargetGroupRequest{Path: "/auth/group/user", HTTPMethod: "GET", MultiValueHeaders: map[string][]string{"access-control-request-headers": {"content-type"}, "access-control-request-method": {"POST"}}})
	assert.Equal(t, 200, response.StatusCode)
	assert.Equal(t, base64.StdEncoding.EncodeToString([]byte("INDEX")), response.Body)
	assert.Nil(t, err)

	response, err = goLA.Register(context.Background(), events.ALBTargetGroupRequest{Path: "/auth/group/noIndex", HTTPMethod: "GET", MultiValueHeaders: map[string][]string{"access-control-request-headers": {"content-type"}, "access-control-request-method": {"POST"}}})
	assert.Equal(t, 200, response.StatusCode)
	assert.Equal(t, base64.StdEncoding.EncodeToString([]byte("GET")), response.Body)
	assert.Nil(t, err)

	response, err = goLA.Register(context.Background(), events.ALBTargetGroupRequest{Path: "/auth/group/bad", HTTPMethod: "GET", MultiValueHeaders: map[string][]string{"access-control-request-headers": {"content-type"}, "access-control-request-method": {"POST"}}})
	assert.Equal(t, 500, response.StatusCode)
	assert.Nil(t, err)

	response, err = goLA.Register(context.Background(), events.ALBTargetGroupRequest{Path: "/auth/group/panic", HTTPMethod: "GET", MultiValueHeaders: map[string][]string{"access-control-request-headers": {"content-type"}, "access-control-request-method": {"POST"}}})
	assert.Equal(t, 500, response.StatusCode)
	assert.Nil(t, err)

	response, err = goLA.Register(context.Background(), events.ALBTargetGroupRequest{Path: "/auth/abc", HTTPMethod: "GET", MultiValueHeaders: map[string][]string{"access-control-request-headers": {"content-type"}, "access-control-request-method": {"POST"}}})
	assert.Equal(t, 404, response.StatusCode)
	assert.Nil(t, err)

	response, err = goLA.Register(context.Background(), events.ALBTargetGroupRequest{Path: "/auth/abc/ccc", HTTPMethod: "GET", MultiValueHeaders: map[string][]string{"access-control-request-headers": {"content-type"}, "access-control-request-method": {"POST"}}})
	assert.Equal(t, 404, response.StatusCode)
	assert.Nil(t, err)

	response, err = goLA.Register(context.Background(), events.ALBTargetGroupRequest{Path: "/auth/abc/cde", HTTPMethod: "GET", MultiValueHeaders: map[string][]string{"access-control-request-headers": {"content-type"}, "access-control-request-method": {"POST"}}})
	assert.Equal(t, 200, response.StatusCode)
	assert.Nil(t, err)

	response, err = goLA.Register(context.Background(), events.ALBTargetGroupRequest{Path: "/auth/abc/cde/123", HTTPMethod: "GET", MultiValueHeaders: map[string][]string{"access-control-request-headers": {"content-type"}, "access-control-request-method": {"POST"}}})
	assert.Equal(t, 200, response.StatusCode)
	assert.Nil(t, err)

}
