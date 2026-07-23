package kafka

import (
	"context"
	"fmt"
	"strings"

	"github.com/fsj00/ops-mcp/internal/config"
	"github.com/fsj00/ops-mcp/internal/model"
	"go.uber.org/zap"
)

// Connector runs Kafka read-only admin ops against named instances from kafka.yaml.
// Go-side entry is Execute(action, params); Runtime exposes typed ctx.kafka.* methods.
type Connector struct {
	cfg *config.Manager
	log *zap.Logger
}

// Result is the Execute response payload (becomes Plugin / Tool data).
type Result map[string]interface{}

func New(cfg *config.Manager, log *zap.Logger) *Connector {
	if log == nil {
		log = zap.NewNop()
	}
	return &Connector{cfg: cfg, log: log}
}

// Supported Execute actions (plugin names are kafka_<action>).
const (
	ActionClusterInfo         = "cluster_info"
	ActionBrokers             = "brokers"
	ActionTopics              = "topics"
	ActionTopicDetail         = "topic_detail"
	ActionPartitionHealth     = "partition_health"
	ActionConsumerGroups      = "consumer_groups"
	ActionConsumerLag         = "consumer_lag"
	ActionConsumerLagSummary  = "consumer_lag_summary"
	ActionTopicOffsets        = "topic_offsets"
	ActionBrokerConfig        = "broker_config"
)

// Execute dispatches a read-only Kafka admin action.
//
// Common params:
//   - kafka (string, required): kafka.yaml instance name
//
// Action-specific params are documented on each handler / Plugin.
func (c *Connector) Execute(ctx context.Context, action string, params map[string]interface{}) (*Result, error) {
	action = strings.TrimSpace(strings.ToLower(action))
	if action == "" {
		return nil, model.NewAppError(model.ErrInvalidParams, "kafka: action is required")
	}
	if !knownAction(action) {
		return nil, model.NewAppError(model.ErrInvalidParams, fmt.Sprintf("kafka: unknown action %q", action))
	}
	if params == nil {
		params = map[string]interface{}{}
	}
	name, err := requireString(params, "kafka")
	if err != nil {
		return nil, err
	}
	// Validate action-specific required params before dialing.
	switch action {
	case ActionTopicDetail, ActionTopicOffsets:
		if _, err := requireString(params, "topic"); err != nil {
			return nil, err
		}
	case ActionConsumerLag:
		if _, err := requireString(params, "group"); err != nil {
			return nil, err
		}
	}

	cl, inst, err := c.dial(ctx, name)
	if err != nil {
		return nil, err
	}
	defer cl.Close()

	c.log.Debug("kafka execute",
		zap.String("kafka", name),
		zap.String("action", action),
	)

	switch action {
	case ActionClusterInfo:
		return c.clusterInfo(ctx, cl)
	case ActionBrokers:
		return c.brokers(ctx, cl)
	case ActionTopics:
		return c.topics(ctx, cl, inst, params)
	case ActionTopicDetail:
		return c.topicDetail(ctx, cl, params)
	case ActionPartitionHealth:
		return c.partitionHealth(ctx, cl, params)
	case ActionConsumerGroups:
		return c.consumerGroups(ctx, cl, inst, params)
	case ActionConsumerLag:
		return c.consumerLag(ctx, cl, params)
	case ActionConsumerLagSummary:
		return c.consumerLagSummary(ctx, cl, inst, params)
	case ActionTopicOffsets:
		return c.topicOffsets(ctx, cl, params)
	case ActionBrokerConfig:
		return c.brokerConfig(ctx, cl, params)
	default:
		return nil, model.NewAppError(model.ErrInvalidParams, fmt.Sprintf("kafka: unknown action %q", action))
	}
}

func knownAction(action string) bool {
	switch action {
	case ActionClusterInfo, ActionBrokers, ActionTopics, ActionTopicDetail,
		ActionPartitionHealth, ActionConsumerGroups, ActionConsumerLag,
		ActionConsumerLagSummary, ActionTopicOffsets, ActionBrokerConfig:
		return true
	default:
		return false
	}
}
