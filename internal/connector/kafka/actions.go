package kafka

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/fsj00/ops-mcp/internal/model"
	"github.com/twmb/franz-go/pkg/kadm"
)

func (c *Connector) clusterInfo(ctx context.Context, cl *client) (*Result, error) {
	meta, err := cl.adm.BrokerMetadata(ctx)
	if err != nil {
		return nil, connErr("cluster_info", err)
	}
	return &Result{
		"cluster_id":    meta.Cluster,
		"controller_id": meta.Controller,
		"broker_count":  len(meta.Brokers),
		"brokers":       brokersToMaps(meta.Brokers),
	}, nil
}

func (c *Connector) brokers(ctx context.Context, cl *client) (*Result, error) {
	meta, err := cl.adm.BrokerMetadata(ctx)
	if err != nil {
		return nil, connErr("brokers", err)
	}
	list := brokersToMaps(meta.Brokers)
	return &Result{
		"brokers":       list,
		"count":         len(list),
		"controller_id": meta.Controller,
		"cluster_id":    meta.Cluster,
	}, nil
}

func (c *Connector) topics(ctx context.Context, cl *client, inst model.KafkaInstance, params map[string]interface{}) (*Result, error) {
	reqLimit, _, err := paramInt(params, "limit")
	if err != nil {
		return nil, err
	}
	prefix := paramString(params, "prefix")
	includeInternal := paramBool(params, "include_internal")

	details, err := cl.adm.ListTopicsWithInternal(ctx)
	if err != nil {
		return nil, connErr("topics", err)
	}

	names := details.Names()
	sort.Strings(names)
	filtered := make([]string, 0, len(names))
	for _, name := range names {
		d := details[name]
		if !includeInternal && d.IsInternal {
			continue
		}
		if prefix != "" && !strings.HasPrefix(name, prefix) {
			continue
		}
		filtered = append(filtered, name)
	}

	limit, truncated := applyLimit(len(filtered), inst.Limit, reqLimit)
	out := make([]map[string]interface{}, 0, limit)
	for _, name := range filtered[:limit] {
		d := details[name]
		out = append(out, map[string]interface{}{
			"topic":           name,
			"partitions":      len(d.Partitions),
			"is_internal":     d.IsInternal,
			"replication_factor": d.Partitions.NumReplicas(),
		})
	}
	return &Result{
		"topics":    out,
		"count":     len(out),
		"total":     len(filtered),
		"truncated": truncated,
	}, nil
}

func (c *Connector) topicDetail(ctx context.Context, cl *client, params map[string]interface{}) (*Result, error) {
	topic, err := requireString(params, "topic")
	if err != nil {
		return nil, err
	}
	meta, err := cl.adm.Metadata(ctx, topic)
	if err != nil {
		return nil, connErr("topic_detail", err)
	}
	d, ok := meta.Topics[topic]
	if !ok {
		return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("kafka: topic %q not found", topic))
	}
	if d.Err != nil {
		return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("kafka: topic %q: %v", topic, d.Err))
	}

	parts := make([]map[string]interface{}, 0, len(d.Partitions))
	underReplicated := 0
	offline := 0
	for _, p := range d.Partitions.Sorted() {
		isrCount := len(p.ISR)
		replicaCount := len(p.Replicas)
		ur := replicaCount > 0 && isrCount < replicaCount
		if ur {
			underReplicated++
		}
		if len(p.OfflineReplicas) > 0 || p.Leader < 0 {
			offline++
		}
		errMsg := ""
		if p.Err != nil {
			errMsg = p.Err.Error()
		}
		parts = append(parts, map[string]interface{}{
			"partition":        p.Partition,
			"leader":           p.Leader,
			"leader_epoch":     p.LeaderEpoch,
			"replicas":         int32Slice(p.Replicas),
			"isr":              int32Slice(p.ISR),
			"offline_replicas": int32Slice(p.OfflineReplicas),
			"under_replicated": ur,
			"error":            errMsg,
		})
	}
	return &Result{
		"topic":                 topic,
		"topic_id":              d.ID.String(),
		"is_internal":           d.IsInternal,
		"partition_count":       len(parts),
		"replication_factor":      d.Partitions.NumReplicas(),
		"under_replicated_count": underReplicated,
		"offline_partition_count": offline,
		"partitions":            parts,
	}, nil
}

func (c *Connector) partitionHealth(ctx context.Context, cl *client, params map[string]interface{}) (*Result, error) {
	topic := paramString(params, "topic")
	var meta kadm.Metadata
	var err error
	if topic != "" {
		meta, err = cl.adm.Metadata(ctx, topic)
	} else {
		meta, err = cl.adm.Metadata(ctx)
	}
	if err != nil {
		return nil, connErr("partition_health", err)
	}

	type issue struct {
		Topic     string `json:"topic"`
		Partition int32  `json:"partition"`
		Leader    int32  `json:"leader"`
		Replicas  int    `json:"replicas"`
		ISR       int    `json:"isr"`
		Offline   int    `json:"offline_replicas"`
		Reason    string `json:"reason"`
	}
	issues := make([]issue, 0)
	totalPartitions := 0
	underReplicated := 0
	offlineParts := 0
	noLeader := 0

	topics := meta.Topics.Sorted()
	for _, d := range topics {
		if d.Err != nil {
			continue
		}
		if topic != "" && d.Topic != topic {
			continue
		}
		for _, p := range d.Partitions.Sorted() {
			totalPartitions++
			reasons := make([]string, 0, 3)
			if p.Leader < 0 {
				noLeader++
				reasons = append(reasons, "no_leader")
			}
			if len(p.OfflineReplicas) > 0 {
				offlineParts++
				reasons = append(reasons, "offline_replicas")
			}
			if len(p.Replicas) > 0 && len(p.ISR) < len(p.Replicas) {
				underReplicated++
				reasons = append(reasons, "under_replicated")
			}
			if p.Err != nil {
				reasons = append(reasons, p.Err.Error())
			}
			if len(reasons) == 0 {
				continue
			}
			issues = append(issues, issue{
				Topic:     d.Topic,
				Partition: p.Partition,
				Leader:    p.Leader,
				Replicas:  len(p.Replicas),
				ISR:       len(p.ISR),
				Offline:   len(p.OfflineReplicas),
				Reason:    strings.Join(reasons, ","),
			})
		}
	}

	healthy := underReplicated == 0 && offlineParts == 0 && noLeader == 0
	return &Result{
		"healthy":                 healthy,
		"topic_count":             len(topics),
		"partition_count":         totalPartitions,
		"under_replicated_count":  underReplicated,
		"offline_partition_count": offlineParts,
		"no_leader_count":         noLeader,
		"issues":                  issues,
		"issue_count":             len(issues),
	}, nil
}

func (c *Connector) consumerGroups(ctx context.Context, cl *client, inst model.KafkaInstance, params map[string]interface{}) (*Result, error) {
	reqLimit, _, err := paramInt(params, "limit")
	if err != nil {
		return nil, err
	}
	listed, err := cl.adm.ListGroups(ctx)
	if err != nil {
		return nil, connErr("consumer_groups", err)
	}
	names := listed.Groups()
	sort.Strings(names)
	limit, truncated := applyLimit(len(names), inst.Limit, reqLimit)

	selected := names[:limit]
	described, descErr := cl.adm.DescribeGroups(ctx, selected...)
	if descErr != nil && described == nil {
		return nil, connErr("consumer_groups", descErr)
	}

	out := make([]map[string]interface{}, 0, len(selected))
	for _, name := range selected {
		item := map[string]interface{}{
			"group": name,
		}
		if g, ok := described[name]; ok {
			item["state"] = g.State
			item["protocol_type"] = g.ProtocolType
			item["protocol"] = g.Protocol
			item["member_count"] = len(g.Members)
			item["coordinator_id"] = g.Coordinator.NodeID
			if g.Err != nil {
				item["error"] = g.Err.Error()
			}
		} else if lg, ok := listed[name]; ok {
			item["state"] = lg.State
			item["protocol_type"] = lg.ProtocolType
			item["coordinator_id"] = lg.Coordinator
		}
		out = append(out, item)
	}
	return &Result{
		"groups":    out,
		"count":     len(out),
		"total":     len(names),
		"truncated": truncated,
	}, nil
}

func (c *Connector) consumerLag(ctx context.Context, cl *client, params map[string]interface{}) (*Result, error) {
	group, err := requireString(params, "group")
	if err != nil {
		return nil, err
	}
	topicFilter := paramString(params, "topic")

	lags, err := cl.adm.Lag(ctx, group)
	if err != nil {
		return nil, connErr("consumer_lag", err)
	}
	gl, ok := lags[group]
	if !ok {
		return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("kafka: consumer group %q not found", group))
	}
	if gl.Error() != nil {
		return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("kafka: consumer group %q: %v", group, gl.Error()))
	}

	partitions := make([]map[string]interface{}, 0)
	var totalLag int64
	for _, ml := range gl.Lag.Sorted() {
		if topicFilter != "" && ml.Topic != topicFilter {
			continue
		}
		if ml.Lag > 0 {
			totalLag += ml.Lag
		}
		errMsg := ""
		if ml.Err != nil {
			errMsg = ml.Err.Error()
		}
		partitions = append(partitions, map[string]interface{}{
			"topic":         ml.Topic,
			"partition":     ml.Partition,
			"committed":     ml.Commit.At,
			"log_start":     ml.Start.Offset,
			"log_end":       ml.End.Offset,
			"lag":           ml.Lag,
			"error":         errMsg,
		})
	}
	return &Result{
		"group":           group,
		"state":           gl.State,
		"protocol":        gl.Protocol,
		"protocol_type":   gl.ProtocolType,
		"member_count":    len(gl.Members),
		"coordinator_id":  gl.Coordinator.NodeID,
		"total_lag":       totalLag,
		"partition_count": len(partitions),
		"partitions":      partitions,
	}, nil
}

func (c *Connector) consumerLagSummary(ctx context.Context, cl *client, inst model.KafkaInstance, params map[string]interface{}) (*Result, error) {
	groupFilter := paramString(params, "group")
	reqLimit, _, err := paramInt(params, "limit")
	if err != nil {
		return nil, err
	}

	var groups []string
	if groupFilter != "" {
		groups = []string{groupFilter}
	} else {
		listed, err := cl.adm.ListGroups(ctx)
		if err != nil {
			return nil, connErr("consumer_lag_summary", err)
		}
		groups = listed.Groups()
		sort.Strings(groups)
		limit, _ := applyLimit(len(groups), inst.Limit, reqLimit)
		groups = groups[:limit]
	}
	if len(groups) == 0 {
		return &Result{
			"groups":      []interface{}{},
			"count":       0,
			"total_lag":   int64(0),
			"group_count": 0,
		}, nil
	}

	lags, err := cl.adm.Lag(ctx, groups...)
	if err != nil {
		return nil, connErr("consumer_lag_summary", err)
	}

	summaries := make([]map[string]interface{}, 0, len(groups))
	var clusterLag int64
	for _, name := range groups {
		gl, ok := lags[name]
		item := map[string]interface{}{
			"group": name,
		}
		if !ok {
			item["error"] = "not found"
			summaries = append(summaries, item)
			continue
		}
		item["state"] = gl.State
		item["member_count"] = len(gl.Members)
		item["coordinator_id"] = gl.Coordinator.NodeID
		if gl.Error() != nil {
			item["error"] = gl.Error().Error()
			item["total_lag"] = int64(-1)
		} else {
			total := gl.Lag.Total()
			item["total_lag"] = total
			item["topic_count"] = len(gl.Lag)
			if total > 0 {
				clusterLag += total
			}
		}
		summaries = append(summaries, item)
	}
	return &Result{
		"groups":      summaries,
		"count":       len(summaries),
		"group_count": len(summaries),
		"total_lag":   clusterLag,
	}, nil
}

func (c *Connector) topicOffsets(ctx context.Context, cl *client, params map[string]interface{}) (*Result, error) {
	topic, err := requireString(params, "topic")
	if err != nil {
		return nil, err
	}
	starts, err := cl.adm.ListStartOffsets(ctx, topic)
	if err != nil {
		return nil, connErr("topic_offsets", err)
	}
	ends, err := cl.adm.ListEndOffsets(ctx, topic)
	if err != nil {
		return nil, connErr("topic_offsets", err)
	}

	startParts := starts[topic]
	endParts := ends[topic]
	partSet := map[int32]struct{}{}
	for p := range startParts {
		partSet[p] = struct{}{}
	}
	for p := range endParts {
		partSet[p] = struct{}{}
	}
	partNums := make([]int32, 0, len(partSet))
	for p := range partSet {
		partNums = append(partNums, p)
	}
	sort.Slice(partNums, func(i, j int) bool { return partNums[i] < partNums[j] })

	partitions := make([]map[string]interface{}, 0, len(partNums))
	var totalMessages int64
	for _, p := range partNums {
		startOff := int64(-1)
		endOff := int64(-1)
		errMsg := ""
		if s, ok := startParts[p]; ok {
			startOff = s.Offset
			if s.Err != nil {
				errMsg = s.Err.Error()
			}
		}
		if e, ok := endParts[p]; ok {
			endOff = e.Offset
			if e.Err != nil {
				if errMsg != "" {
					errMsg += "; "
				}
				errMsg += e.Err.Error()
			}
		}
		var size int64 = -1
		if startOff >= 0 && endOff >= startOff {
			size = endOff - startOff
			totalMessages += size
		}
		partitions = append(partitions, map[string]interface{}{
			"partition":    p,
			"earliest":     startOff,
			"latest":       endOff,
			"message_count": size,
			"error":        errMsg,
		})
	}
	return &Result{
		"topic":          topic,
		"partitions":     partitions,
		"partition_count": len(partitions),
		"total_messages": totalMessages,
	}, nil
}

func (c *Connector) brokerConfig(ctx context.Context, cl *client, params map[string]interface{}) (*Result, error) {
	brokerID, hasID, err := paramInt(params, "broker_id")
	if err != nil {
		return nil, err
	}
	prefix := paramString(params, "prefix")

	var configs kadm.ResourceConfigs
	if hasID {
		configs, err = cl.adm.DescribeBrokerConfigs(ctx, int32(brokerID))
	} else {
		// Empty broker list returns cluster-level / any-broker dynamic configs.
		configs, err = cl.adm.DescribeBrokerConfigs(ctx)
	}
	if err != nil {
		return nil, connErr("broker_config", err)
	}

	out := make([]map[string]interface{}, 0, len(configs))
	for _, rc := range configs {
		entries := make([]map[string]interface{}, 0, len(rc.Configs))
		for _, cfg := range rc.Configs {
			if prefix != "" && !strings.HasPrefix(cfg.Key, prefix) {
				continue
			}
			val := ""
			if cfg.Sensitive {
				val = "[REDACTED]"
			} else {
				val = cfg.MaybeValue()
			}
			entries = append(entries, map[string]interface{}{
				"name":      cfg.Key,
				"value":     val,
				"sensitive": cfg.Sensitive,
				"source":    int(cfg.Source),
			})
		}
		sort.Slice(entries, func(i, j int) bool {
			return entries[i]["name"].(string) < entries[j]["name"].(string)
		})
		item := map[string]interface{}{
			"broker":  rc.Name,
			"configs": entries,
			"count":   len(entries),
		}
		if rc.Err != nil {
			item["error"] = rc.Err.Error()
		}
		out = append(out, item)
	}
	return &Result{
		"brokers": out,
		"count":   len(out),
	}, nil
}

func brokersToMaps(brokers kadm.BrokerDetails) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(brokers))
	for _, b := range brokers {
		out = append(out, map[string]interface{}{
			"id":   b.NodeID,
			"host": b.Host,
			"port": b.Port,
			"rack": b.Rack,
		})
	}
	return out
}

func int32Slice(in []int32) []int {
	out := make([]int, len(in))
	for i, v := range in {
		out[i] = int(v)
	}
	return out
}

func connErr(action string, err error) error {
	return model.NewAppError(model.ErrConnectorError, fmt.Sprintf("kafka %s: %v", action, err))
}
