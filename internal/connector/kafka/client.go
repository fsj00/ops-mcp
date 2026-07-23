package kafka

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/fsj00/ops-mcp/internal/model"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"
)

// client wraps a franz-go client + admin client for one Execute call.
type client struct {
	kgo *kgo.Client
	adm *kadm.Client
}

func (c *client) Close() {
	if c == nil || c.kgo == nil {
		return
	}
	c.kgo.Close()
}

func (c *Connector) dial(ctx context.Context, name string) (*client, model.KafkaInstance, error) {
	inst, err := c.lookup(name)
	if err != nil {
		return nil, model.KafkaInstance{}, err
	}
	opts, err := clientOpts(inst)
	if err != nil {
		return nil, model.KafkaInstance{}, err
	}
	cl, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, model.KafkaInstance{}, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("kafka dial %q: %v", name, err))
	}
	// Force a metadata refresh so connection errors surface early.
	if err := cl.Ping(ctx); err != nil {
		cl.Close()
		return nil, model.KafkaInstance{}, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("kafka connect %q: %v", name, err))
	}
	return &client{kgo: cl, adm: kadm.NewClient(cl)}, inst, nil
}

func (c *Connector) lookup(name string) (model.KafkaInstance, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return model.KafkaInstance{}, model.NewAppError(model.ErrInvalidParams, "kafka: kafka name is required")
	}
	inst, err := c.cfg.GetKafka(name)
	if err != nil {
		return model.KafkaInstance{}, model.NewAppError(model.ErrConnectorError, err.Error())
	}
	return inst, nil
}

func clientOpts(inst model.KafkaInstance) ([]kgo.Opt, error) {
	brokers := make([]string, 0, len(inst.Connection.Brokers))
	for _, b := range inst.Connection.Brokers {
		b = strings.TrimSpace(b)
		if b != "" {
			brokers = append(brokers, b)
		}
	}
	if len(brokers) == 0 {
		return nil, model.NewAppError(model.ErrInvalidParams, fmt.Sprintf("kafka %q: connection.brokers is empty", inst.Name))
	}

	opts := []kgo.Opt{
		kgo.SeedBrokers(brokers...),
		kgo.ClientID("ops-mcp"),
	}

	serverName := ""
	if host, _, err := net.SplitHostPort(brokers[0]); err == nil {
		serverName = host
	} else {
		serverName = brokers[0]
	}

	tlsCfg, err := buildTLSConfig(serverName, inst.Connection.TLS)
	if err != nil {
		return nil, model.NewAppError(model.ErrInvalidParams, err.Error())
	}
	if tlsCfg != nil {
		opts = append(opts, kgo.DialTLSConfig(tlsCfg))
	}

	mech, err := saslMechanism(inst.Connection.SASL)
	if err != nil {
		return nil, err
	}
	if mech != nil {
		opts = append(opts, kgo.SASL(mech))
	}
	return opts, nil
}

func saslMechanism(cfg model.KafkaSASL) (sasl.Mechanism, error) {
	mech := strings.ToLower(strings.TrimSpace(cfg.Mechanism))
	user := strings.TrimSpace(cfg.Username)
	pass := cfg.Password
	if mech == "" && user == "" && pass == "" {
		return nil, nil
	}
	if mech == "" {
		mech = "plain"
	}
	if user == "" {
		return nil, model.NewAppError(model.ErrInvalidParams, "kafka sasl: username is required when SASL is enabled")
	}
	switch mech {
	case "plain":
		return plain.Auth{User: user, Pass: pass}.AsMechanism(), nil
	case "scram-sha-256", "scram_sha_256", "sha256":
		return scram.Auth{User: user, Pass: pass}.AsSha256Mechanism(), nil
	case "scram-sha-512", "scram_sha_512", "sha512":
		return scram.Auth{User: user, Pass: pass}.AsSha512Mechanism(), nil
	default:
		return nil, model.NewAppError(model.ErrInvalidParams, fmt.Sprintf("kafka sasl: unsupported mechanism %q (plain|scram-sha-256|scram-sha-512)", cfg.Mechanism))
	}
}
