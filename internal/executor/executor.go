package executor

import (
	"context"
	"fmt"
	"time"

	"github.com/fsj00/ops-mcp/internal/model"
	"github.com/fsj00/ops-mcp/internal/runtime"
)

// Executor validates params and runs plugins.
type Executor struct {
	rt              *runtime.Runtime
	defaultTimeout  time.Duration
}

func New(rt *runtime.Runtime, defaultTimeout time.Duration) *Executor {
	if defaultTimeout <= 0 {
		defaultTimeout = 30 * time.Second
	}
	return &Executor{rt: rt, defaultTimeout: defaultTimeout}
}

// Execute validates arguments and runs the plugin script.
func (e *Executor) Execute(ctx context.Context, plugin *model.PluginMeta, args map[string]interface{}) model.ToolResult {
	if args == nil {
		args = map[string]interface{}{}
	}
	if err := validateParams(plugin, args); err != nil {
		return model.FailResult(model.ErrInvalidParams, err.Error())
	}

	timeout := plugin.TimeoutDuration(e.defaultTimeout)
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	data, err := e.rt.Execute(execCtx, plugin.Script, args, timeout)
	if err != nil {
		if ae, ok := err.(*model.AppError); ok {
			return model.FailResult(ae.Code, ae.Message)
		}
		return model.FailResult(model.ErrInternalError, err.Error())
	}
	return model.SuccessResult(data)
}

func validateParams(plugin *model.PluginMeta, args map[string]interface{}) error {
	for name, field := range plugin.Input {
		val, ok := args[name]
		if field.Required && (!ok || val == nil || val == "") {
			return fmt.Errorf("missing required param %q", name)
		}
		if !ok || val == nil {
			continue
		}
		if err := checkType(name, field.Type, val); err != nil {
			return err
		}
	}
	return nil
}

func checkType(name, typ string, val interface{}) error {
	switch typ {
	case "string":
		if _, ok := val.(string); !ok {
			return fmt.Errorf("param %q must be string", name)
		}
	case "number":
		switch val.(type) {
		case float64, float32, int, int32, int64:
			return nil
		default:
			return fmt.Errorf("param %q must be number", name)
		}
	case "boolean":
		if _, ok := val.(bool); !ok {
			return fmt.Errorf("param %q must be boolean", name)
		}
	case "array":
		switch val.(type) {
		case []interface{}, []string:
			return nil
		default:
			return fmt.Errorf("param %q must be array", name)
		}
	case "object":
		if _, ok := val.(map[string]interface{}); !ok {
			return fmt.Errorf("param %q must be object", name)
		}
	}
	return nil
}
