package gola

import (
	"encoding/base64"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	httpheadername "github.com/kklab-com/gone-httpheadername"
	httpstatus "github.com/kklab-com/gone-httpstatus"
	buf "github.com/kklab-com/goth-bytebuf"
)

type Request interface {
	Request() *events.ALBTargetGroupRequest
	Method() string
	Path() string
	PathParameter(name string) string
	TraceId() string
	UserAgent() string
	Header() http.Header
	GetHeader(name string) string
	GetHeaders(name string) []string
	QueryValue(name string) string
	QueryValues(name string) []string
	Body() buf.ByteBuf
}

type request struct {
	base           *events.ALBTargetGroupRequest
	pathParameters map[string]string
}

func (r *request) Request() *events.ALBTargetGroupRequest {
	return r.base
}

func newRequest(req events.ALBTargetGroupRequest, pathParameters map[string]string) Request {
	return &request{base: &req, pathParameters: pathParameters}
}

func (r *request) Method() string {
	return r.base.HTTPMethod
}

func (r *request) Path() string {
	return r.base.Path
}

func (r *request) PathParameter(name string) string {
	if v, f := r.pathParameters[name]; f {
		return v
	}

	return ""
}

func (r *request) TraceId() string {
	return r.GetHeader("x-amzn-trace-id")
}

func (r *request) Header() http.Header {
	return r.base.MultiValueHeaders
}

func (r *request) GetHeader(name string) string {
	return http.Header(r.base.MultiValueHeaders).Get(name)
}

func (r *request) GetHeaders(name string) []string {
	return http.Header(r.base.MultiValueHeaders).Values(name)
}

func (r *request) QueryValue(name string) string {
	return r.base.QueryStringParameters[name]
}

func (r *request) QueryValues(name string) []string {
	return r.base.MultiValueQueryStringParameters[name]
}

func (r *request) Body() buf.ByteBuf {
	if r.base.Body == "" {
		return buf.EmptyByteBuf()
	}

	if r.base.IsBase64Encoded {
		decodeString, err := base64.StdEncoding.DecodeString(r.base.Body)
		if err != nil {
			return buf.EmptyByteBuf()
		}

		return buf.NewByteBuf(decodeString)
	} else {
		return buf.NewByteBufString(r.base.Body)
	}
}

func (r *request) UserAgent() string {
	return r.Header().Get("User-Agent")
}

type Response interface {
	Build() *events.ALBTargetGroupResponse
	StatusCode() int
	SetStatusCode(code int) Response
	AddHeader(name string, value string) Response
	SetHeader(name string, value string) Response
	DelHeader(name string) Response
	Header() http.Header
	GetHeader(name string) string
	GetHeaders(name string) []string
	Cookie(name string) *http.Cookie
	SetCookie(cookie http.Cookie) Response
	Cookies() map[string][]http.Cookie
	Body() []byte
	SetBody(buf buf.ByteBuf) Response
	SetContentType(ct string) Response
	JSONResponse(buf buf.ByteBuf) Response
}

type response struct {
	code    int
	headers http.Header
	cookies map[string][]http.Cookie
	body    buf.ByteBuf
}

func newResponse() Response {
	return &response{
		code:    0,
		headers: map[string][]string{},
		cookies: map[string][]http.Cookie{},
		body:    buf.EmptyByteBuf(),
	}
}

func (r *response) Build() *events.ALBTargetGroupResponse {
	albResp := &events.ALBTargetGroupResponse{}
	for _, value := range r.cookies {
		for _, cookie := range value {
			if v := cookie.String(); v != "" {
				r.headers.Add("Set-Cookie", v)
			}
		}
	}

	code := r.code
	if code == 0 {
		code = 200
	}

	albResp.Headers = map[string]string{}
	albResp.MultiValueHeaders = r.headers
	albResp.StatusCode = code
	albResp.Body = base64.StdEncoding.EncodeToString(r.body.Bytes())
	albResp.IsBase64Encoded = true
	return albResp
}

func (r *response) Redirect(redirectUrl string) {
	r.SetStatusCode(httpstatus.Found).
		SetHeader(httpheadername.Location, redirectUrl)
}

func (r *response) StatusCode() int {
	return r.code
}

func (r *response) SetStatusCode(code int) Response {
	r.code = code
	return r
}

func (r *response) AddHeader(name string, value string) Response {
	r.headers.Add(name, value)
	return r
}

func (r *response) SetHeader(name string, value string) Response {
	r.headers.Set(name, value)
	return r
}

func (r *response) DelHeader(name string) Response {
	r.headers.Del(name)
	return r
}

func (r *response) Header() http.Header {
	return r.headers
}

func (r *response) GetHeader(name string) string {
	return r.headers.Get(name)
}

func (r *response) GetHeaders(name string) []string {
	return r.headers.Values(name)
}

func (r *response) Cookie(name string) *http.Cookie {
	if cookie, f := r.cookies[name]; f {
		return &cookie[0]
	}

	return nil
}

func (r *response) SetCookie(cookie http.Cookie) Response {
	r.cookies[cookie.Name] = []http.Cookie{cookie}
	return r
}

func (r *response) Cookies() map[string][]http.Cookie {
	return r.cookies
}

func (r *response) Body() []byte {
	return r.body.Bytes()
}

func (r *response) SetBody(buf buf.ByteBuf) Response {
	r.body = buf
	return r
}

func (r *response) SetContentType(contentType string) Response {
	r.SetHeader(httpheadername.ContentType, contentType)
	return r
}

func (r *response) JSONResponse(buf buf.ByteBuf) Response {
	return r.
		SetHeader(httpheadername.ContentType, "application/json").
		SetBody(buf)
}
