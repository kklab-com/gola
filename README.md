# GoLA(Golang framework for Lambda with ALB)

## HOWTO

### Enable ALB MultiValueSupport

[Enable Multi Value Header](https://docs.aws.amazon.com/elasticloadbalancing/latest/application/lambda-functions.html#enable-multi-value-headers)

### New Serve

```go
serve := gola.NewServe()
```

### Register Serve
```go
serve := gola.NewServe()
runtime.Start(serve.Register)
```

### Register Endpoint
```go
type DefaultRootHandler struct {
}

func (d *DefaultRootHandler) Run(ctx context.Context, request Request, response Response) (er error) {
    response.SetHeader("ROOT", "VAL")
    response.SetBody(buf.EmptyByteBuf().WriteString("{}"))
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
    response.SetHeader("USER_ID", request.PathParameter("user_id"))
    response.SetHeader("BOOK_ID", request.PathParameter("book"))
    response.JSONResponse(buf.EmptyByteBuf().WriteString("{}"))
    return nil
}

type DefaultBadHandler struct {
}

func (d *DefaultBadHandler) Run(ctx context.Context, request Request, response Response) (er error) {
    return erresponse.ServerError
}


func main() {
    serve := gola.NewServe()
    var CORS = &gola.DefaultCORSHandler{}
    route.
        // :user_id is path parameter
        SetEndpoint("/auth/group/user/:user_id", CORS, &DefaultEmptyHandler{}).
        // :user_id, :book are path parameter
        SetEndpoint("/auth/group/user/:user_id/book/:book", CORS, &DefaultJSONHandler{}).
        SetEndpoint("/auth/group/user/:user_id/profile", CORS, &DefaultEmptyHandler{}).
        SetEndpoint("/bad", CORS, &DefaultBadHandler{}).
        SetEndpoint("/wild/*", CORS, &DefaultWildHandler{}).
        SetEndpoint("/case/wild/*", CORS, &DefaultWildHandler{})
    
    runtime.Start(serve.Register)
}
```