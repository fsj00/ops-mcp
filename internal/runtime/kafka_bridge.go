package runtime

import (
	"context"

	kafkaconn "github.com/fsj00/ops-mcp/internal/connector/kafka"
	"github.com/fsj00/ops-mcp/internal/model"
	"github.com/dop251/goja"
)

func (r *Runtime) wrapKafkaList(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if r.deps.Cfg == nil {
			panic(appErrorValue(vm, model.NewAppError(model.ErrConnectorError, "config manager not configured")))
		}
		return vm.ToValue(r.deps.Cfg.ListKafkaSummaries())
	}
}

func (r *Runtime) kafkaConn(vm *goja.Runtime) *kafkaconn.Connector {
	if r.deps.Kafka == nil {
		panic(appErrorValue(vm, model.NewAppError(model.ErrConnectorError, "kafka connector not configured")))
	}
	return r.deps.Kafka
}

func (r *Runtime) wrapKafkaAction(ctx context.Context, vm *goja.Runtime, action string) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		params := map[string]interface{}{}
		if !goja.IsUndefined(call.Argument(0)) && !goja.IsNull(call.Argument(0)) {
			if err := mapToStruct(call.Argument(0).Export(), &params); err != nil {
				panic(appErrorValue(vm, model.NewAppError(model.ErrInvalidParams, err.Error())))
			}
		}
		res, err := r.kafkaConn(vm).Execute(ctx, action, params)
		if err != nil {
			panic(appErrorValue(vm, err))
		}
		if res == nil {
			return vm.ToValue(map[string]interface{}{})
		}
		return vm.ToValue(map[string]interface{}(*res))
	}
}
