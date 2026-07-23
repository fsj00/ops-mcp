package runtime

import (
	"context"
	"fmt"
	"strings"
	"time"

	httpconn "github.com/fsj00/ops-mcp/internal/connector/http"
	"github.com/fsj00/ops-mcp/internal/model"
	"github.com/dop251/goja"
)

// httpCallOptions is the JS options object for ctx.http.*.
type httpCallOptions struct {
	API       string                 `json:"api"`
	URL       string                 `json:"url"`
	Method    string                 `json:"method"`
	Path      string                 `json:"path"`
	Query     map[string]interface{} `json:"query"`
	Headers   map[string]interface{} `json:"headers"`
	Body      interface{}            `json:"body"`
	Timeout   string                 `json:"timeout"`
	VerifyTLS *bool                  `json:"verify_tls"`
}

func (r *Runtime) wrapHTTPRequest(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return r.wrapHTTPMethod(ctx, vm, "")
}

func (r *Runtime) wrapHTTPGet(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return r.wrapHTTPMethod(ctx, vm, "GET")
}

func (r *Runtime) wrapHTTPPost(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return r.wrapHTTPMethod(ctx, vm, "POST")
}

func (r *Runtime) wrapHTTPPut(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return r.wrapHTTPMethod(ctx, vm, "PUT")
}

func (r *Runtime) wrapHTTPPatch(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return r.wrapHTTPMethod(ctx, vm, "PATCH")
}

func (r *Runtime) wrapHTTPDelete(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return r.wrapHTTPMethod(ctx, vm, "DELETE")
}

func (r *Runtime) wrapHTTPMethod(ctx context.Context, vm *goja.Runtime, fixedMethod string) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if r.deps.HTTP == nil {
			panic(appErrorValue(vm, model.NewAppError(model.ErrConnectorError, "http connector not configured")))
		}
		opts := decodeArg[httpCallOptions](vm, call)
		method := strings.ToUpper(strings.TrimSpace(opts.Method))
		if fixedMethod != "" {
			method = fixedMethod
		}
		if method == "" {
			panic(appErrorValue(vm, model.NewAppError(model.ErrInvalidParams, "method is required")))
		}

		var timeout time.Duration
		if opts.Timeout != "" {
			d, err := time.ParseDuration(opts.Timeout)
			if err != nil || d <= 0 {
				panic(appErrorValue(vm, model.NewAppError(model.ErrInvalidParams, fmt.Sprintf("invalid timeout %q", opts.Timeout))))
			}
			timeout = d
		}

		req := httpconn.Request{
			API:       opts.API,
			URL:       opts.URL,
			Method:    method,
			Path:      opts.Path,
			Query:     stringifyMap(opts.Query),
			Headers:   stringifyMap(opts.Headers),
			Body:      opts.Body,
			Timeout:   timeout,
			VerifyTLS: opts.VerifyTLS,
		}
		res, err := r.deps.HTTP.Do(ctx, req)
		if err != nil {
			panic(appErrorValue(vm, err))
		}
		return vm.ToValue(res)
	}
}

func stringifyMap(in map[string]interface{}) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		if v == nil {
			continue
		}
		out[k] = fmt.Sprint(v)
	}
	return out
}
