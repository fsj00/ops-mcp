package runtime

import (
	"context"

	"github.com/fsj00/ops-mcp/internal/config"
	snmpconn "github.com/fsj00/ops-mcp/internal/connector/snmp"
	"github.com/fsj00/ops-mcp/internal/model"
	"github.com/dop251/goja"
)

func (r *Runtime) snmpConn(vm *goja.Runtime) *snmpconn.Connector {
	if r.deps.SNMP == nil {
		panic(appErrorValue(vm, model.NewAppError(model.ErrConnectorError, "snmp connector not configured")))
	}
	return r.deps.SNMP
}

func (r *Runtime) wrapSNMPDevicesList(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if r.deps.Cfg == nil {
			panic(appErrorValue(vm, model.NewAppError(model.ErrConnectorError, "config manager not configured")))
		}
		opts := config.SNMPDeviceListOptions{}
		if !goja.IsUndefined(call.Argument(0)) && !goja.IsNull(call.Argument(0)) {
			var raw struct {
				Labels map[string]string `json:"labels"`
				Limit  int               `json:"limit"`
				Offset int               `json:"offset"`
			}
			if err := mapToStruct(call.Argument(0).Export(), &raw); err != nil {
				panic(appErrorValue(vm, model.NewAppError(model.ErrInvalidParams, err.Error())))
			}
			opts.Labels = raw.Labels
			opts.Limit = raw.Limit
			opts.Offset = raw.Offset
		}
		return vm.ToValue(r.deps.Cfg.ListSNMPDeviceSummaries(opts))
	}
}

func (r *Runtime) wrapSNMPGet(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		req := decodeArg[snmpconn.GetRequest](vm, call)
		res, err := r.snmpConn(vm).Get(ctx, req)
		if err != nil {
			panic(appErrorValue(vm, err))
		}
		return vm.ToValue(res)
	}
}

func (r *Runtime) wrapSNMPWalk(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		req := decodeArg[snmpconn.WalkRequest](vm, call)
		res, err := r.snmpConn(vm).Walk(ctx, req)
		if err != nil {
			panic(appErrorValue(vm, err))
		}
		return vm.ToValue(res)
	}
}

func (r *Runtime) wrapSNMPBulk(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		req := decodeArg[snmpconn.WalkRequest](vm, call)
		res, err := r.snmpConn(vm).Bulk(ctx, req)
		if err != nil {
			panic(appErrorValue(vm, err))
		}
		return vm.ToValue(res)
	}
}
