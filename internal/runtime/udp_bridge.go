package runtime

import (
	"context"

	"github.com/dop251/goja"
	"github.com/fsj00/ops-mcp/internal/connector/netutil"
	udpconn "github.com/fsj00/ops-mcp/internal/connector/udp"
	"github.com/fsj00/ops-mcp/internal/model"
)

func (r *Runtime) udpConn(vm *goja.Runtime) *udpconn.Connector {
	if r.deps.UDP == nil {
		panic(appErrorValue(vm, model.NewAppError(model.ErrConnectorError, "udp connector not configured")))
	}
	return r.deps.UDP
}

func (r *Runtime) wrapUDPExchange(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		req := decodeArg[netutil.ExchangeRequest](vm, call)
		res, err := r.udpConn(vm).Exchange(ctx, req)
		if err != nil {
			panic(appErrorValue(vm, err))
		}
		return vm.ToValue(res)
	}
}
