package snmp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/fsj00/ops-mcp/internal/config"
	"github.com/fsj00/ops-mcp/internal/model"
	"github.com/gosnmp/gosnmp"
	"go.uber.org/zap"
)

const (
	defaultConcurrency = 32
	maxGetOIDs         = 64
)

// Connector runs read-only SNMP ops against named devices from snmp.yaml.
type Connector struct {
	cfg  *config.Manager
	log  *zap.Logger
	sem  chan struct{}
	dial dialFunc // test seam; nil → real gosnmp dial
}

type dialFunc func(ctx context.Context, device model.SNMPDevice, auth model.SNMPAuth, settings deviceSettings) (snmpClient, error)

// snmpClient is the subset of gosnmp used by this connector.
type snmpClient interface {
	Get(oids []string) (*gosnmp.SnmpPacket, error)
	Walk(rootOid string, walkFn gosnmp.WalkFunc) error
	BulkWalk(rootOid string, walkFn gosnmp.WalkFunc) error
	Close() error
}

type deviceSettings struct {
	Timeout        time.Duration
	Retries        int
	MaxRepetitions uint32
	WalkMaxOIDs    int
}

// Var is one SNMP variable binding in Tool/Plugin results.
type Var struct {
	OID   string      `json:"oid"`
	Type  string      `json:"type"`
	Value interface{} `json:"value"`
}

// Result is the unified get/walk/bulk payload.
type Result struct {
	Device    string `json:"device"`
	Vars      []Var  `json:"vars"`
	Truncated bool   `json:"truncated"`
	Count     int    `json:"count"`
}

// GetRequest is ctx.snmp.get input.
type GetRequest struct {
	Device string   `json:"device"`
	OIDs   []string `json:"oids"`
}

// WalkRequest is ctx.snmp.walk / bulk input.
type WalkRequest struct {
	Device         string `json:"device"`
	OID            string `json:"oid"`
	MaxOIDs        int    `json:"max_oids"`
	MaxRepetitions int    `json:"max_repetitions"`
}

func New(cfg *config.Manager, log *zap.Logger) *Connector {
	if log == nil {
		log = zap.NewNop()
	}
	return &Connector{
		cfg: cfg,
		log: log,
		sem: make(chan struct{}, defaultConcurrency),
	}
}

// Get performs SNMP GET for the given OIDs.
func (c *Connector) Get(ctx context.Context, req GetRequest) (*Result, error) {
	device := strings.TrimSpace(req.Device)
	if device == "" {
		return nil, model.NewAppError(model.ErrInvalidParams, "snmp: device is required")
	}
	oids := normalizeOIDs(req.OIDs)
	if len(oids) == 0 {
		return nil, model.NewAppError(model.ErrInvalidParams, "snmp: oids is required")
	}
	if len(oids) > maxGetOIDs {
		return nil, model.NewAppError(model.ErrInvalidParams, fmt.Sprintf("snmp: at most %d oids per get", maxGetOIDs))
	}

	if err := c.acquire(ctx); err != nil {
		return nil, err
	}
	defer c.release()

	cl, name, err := c.open(ctx, device)
	if err != nil {
		return nil, err
	}
	defer cl.Close() //nolint:errcheck

	c.log.Debug("snmp get", zap.String("device", name), zap.Int("oids", len(oids)))

	pkt, err := cl.Get(oids)
	if err != nil {
		return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("snmp get %s: %v", name, err))
	}
	vars := make([]Var, 0, len(pkt.Variables))
	for _, v := range pkt.Variables {
		vars = append(vars, pduVar(v))
	}
	return &Result{Device: name, Vars: vars, Count: len(vars)}, nil
}

// Walk performs SNMP WALK under oid (truncated by walk_max_oids).
func (c *Connector) Walk(ctx context.Context, req WalkRequest) (*Result, error) {
	return c.doWalk(ctx, req, false)
}

// Bulk performs SNMP BULKWALK under oid (v2c/v3).
func (c *Connector) Bulk(ctx context.Context, req WalkRequest) (*Result, error) {
	return c.doWalk(ctx, req, true)
}

func (c *Connector) doWalk(ctx context.Context, req WalkRequest, bulk bool) (*Result, error) {
	device := strings.TrimSpace(req.Device)
	if device == "" {
		return nil, model.NewAppError(model.ErrInvalidParams, "snmp: device is required")
	}
	oid := strings.TrimSpace(req.OID)
	if oid == "" {
		return nil, model.NewAppError(model.ErrInvalidParams, "snmp: oid is required")
	}

	dev, err := c.cfg.GetSNMPDevice(device)
	if err != nil {
		return nil, model.NewAppError(model.ErrInvalidParams, err.Error())
	}
	settings := c.resolveSettings(dev)
	maxOIDs := settings.WalkMaxOIDs
	if req.MaxOIDs > 0 && req.MaxOIDs < maxOIDs {
		maxOIDs = req.MaxOIDs
	}
	if req.MaxRepetitions > 0 {
		settings.MaxRepetitions = uint32(req.MaxRepetitions)
	}

	if err := c.acquire(ctx); err != nil {
		return nil, err
	}
	defer c.release()

	cl, name, err := c.openDevice(ctx, dev, settings)
	if err != nil {
		return nil, err
	}
	defer cl.Close() //nolint:errcheck

	op := "walk"
	if bulk {
		op = "bulk"
	}
	c.log.Debug("snmp "+op, zap.String("device", name), zap.String("oid", oid), zap.Int("max_oids", maxOIDs))

	vars := make([]Var, 0, 32)
	truncated := false
	walkFn := func(pdu gosnmp.SnmpPDU) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if len(vars) >= maxOIDs {
			truncated = true
			return fmt.Errorf("walk truncated at %d oids", maxOIDs)
		}
		vars = append(vars, pduVar(pdu))
		return nil
	}

	var walkErr error
	if bulk {
		walkErr = cl.BulkWalk(oid, walkFn)
	} else {
		walkErr = cl.Walk(oid, walkFn)
	}
	if walkErr != nil && !truncated {
		// gosnmp may surface our truncation sentinel; treat only real errors as failures.
		if ctx.Err() != nil {
			return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("snmp %s %s: %v", op, name, ctx.Err()))
		}
		if !strings.Contains(walkErr.Error(), "walk truncated") {
			return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("snmp %s %s: %v", op, name, walkErr))
		}
		truncated = true
	}
	return &Result{Device: name, Vars: vars, Truncated: truncated, Count: len(vars)}, nil
}

func (c *Connector) open(ctx context.Context, deviceName string) (snmpClient, string, error) {
	dev, err := c.cfg.GetSNMPDevice(deviceName)
	if err != nil {
		return nil, "", model.NewAppError(model.ErrInvalidParams, err.Error())
	}
	cl, name, err := c.openDevice(ctx, dev, c.resolveSettings(dev))
	return cl, name, err
}

func (c *Connector) acquire(ctx context.Context) error {
	select {
	case c.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return model.NewAppError(model.ErrConnectorError, fmt.Sprintf("snmp: %v", ctx.Err()))
	}
}

func (c *Connector) release() {
	<-c.sem
}

func (c *Connector) openDevice(ctx context.Context, dev model.SNMPDevice, settings deviceSettings) (snmpClient, string, error) {
	auth, err := c.cfg.ResolveSNMPAuth(dev.Name)
	if err != nil {
		return nil, "", model.NewAppError(model.ErrInvalidParams, err.Error())
	}

	dial := c.dial
	if dial == nil {
		dial = dialGoSNMP
	}
	cl, err := dial(ctx, dev, auth, settings)
	if err != nil {
		return nil, "", model.NewAppError(model.ErrConnectorError, fmt.Sprintf("snmp connect %s: %v", dev.Name, err))
	}
	return cl, dev.Name, nil
}

func (c *Connector) resolveSettings(dev model.SNMPDevice) deviceSettings {
	defs := c.cfg.SNMPDefaults()
	timeoutStr := defs.Timeout
	if dev.Timeout != "" {
		timeoutStr = dev.Timeout
	}
	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil || timeout <= 0 {
		timeout = 5 * time.Second
	}
	retries := defs.Retries
	if dev.Retries != nil {
		retries = *dev.Retries
	}
	if retries < 0 {
		retries = 0
	}
	maxRep := defs.MaxRepetitions
	if dev.MaxRepetitions != nil && *dev.MaxRepetitions > 0 {
		maxRep = *dev.MaxRepetitions
	}
	walkMax := defs.WalkMaxOIDs
	if dev.WalkMaxOIDs != nil && *dev.WalkMaxOIDs > 0 {
		walkMax = *dev.WalkMaxOIDs
	}
	if walkMax <= 0 {
		walkMax = config.DefaultSNMPWalkMaxOIDs
	}
	return deviceSettings{
		Timeout:        timeout,
		Retries:        retries,
		MaxRepetitions: uint32(maxRep),
		WalkMaxOIDs:    walkMax,
	}
}

func normalizeOIDs(oids []string) []string {
	out := make([]string, 0, len(oids))
	for _, o := range oids {
		o = strings.TrimSpace(o)
		if o != "" {
			out = append(out, o)
		}
	}
	return out
}

func pduVar(pdu gosnmp.SnmpPDU) Var {
	return Var{
		OID:   pdu.Name,
		Type:  pdu.Type.String(),
		Value: pduValue(pdu),
	}
}

func pduValue(pdu gosnmp.SnmpPDU) interface{} {
	switch pdu.Type {
	case gosnmp.OctetString:
		if b, ok := pdu.Value.([]byte); ok {
			return string(b)
		}
		return pdu.Value
	case gosnmp.ObjectIdentifier:
		if s, ok := pdu.Value.(string); ok {
			return s
		}
		return fmt.Sprint(pdu.Value)
	case gosnmp.Null, gosnmp.NoSuchObject, gosnmp.NoSuchInstance, gosnmp.EndOfMibView:
		return nil
	default:
		return pdu.Value
	}
}

type goSNMPClient struct {
	g *gosnmp.GoSNMP
}

func (c *goSNMPClient) Get(oids []string) (*gosnmp.SnmpPacket, error) {
	return c.g.Get(oids)
}

func (c *goSNMPClient) Walk(rootOid string, walkFn gosnmp.WalkFunc) error {
	return c.g.Walk(rootOid, walkFn)
}

func (c *goSNMPClient) BulkWalk(rootOid string, walkFn gosnmp.WalkFunc) error {
	return c.g.BulkWalk(rootOid, walkFn)
}

func (c *goSNMPClient) Close() error {
	if c.g != nil && c.g.Conn != nil {
		return c.g.Conn.Close()
	}
	return nil
}

func dialGoSNMP(ctx context.Context, device model.SNMPDevice, auth model.SNMPAuth, settings deviceSettings) (snmpClient, error) {
	g, err := buildGoSNMP(ctx, device, auth, settings)
	if err != nil {
		return nil, err
	}
	if err := g.Connect(); err != nil {
		return nil, err
	}
	return &goSNMPClient{g: g}, nil
}

func buildGoSNMP(ctx context.Context, device model.SNMPDevice, auth model.SNMPAuth, settings deviceSettings) (*gosnmp.GoSNMP, error) {
	g := &gosnmp.GoSNMP{
		Target:             device.Address.Host,
		Port:               uint16(device.Address.Port),
		Timeout:            settings.Timeout,
		Retries:            settings.Retries,
		MaxRepetitions:     settings.MaxRepetitions,
		ExponentialTimeout: false,
		Context:            ctx,
		ContextName:        device.Context,
	}

	switch strings.ToLower(strings.TrimSpace(auth.Version)) {
	case "2c":
		g.Version = gosnmp.Version2c
		g.Community = auth.Community
	case "3":
		g.Version = gosnmp.Version3
		g.SecurityModel = gosnmp.UserSecurityModel
		msgFlags, err := snmpMsgFlags(auth.SecurityLevel)
		if err != nil {
			return nil, err
		}
		g.MsgFlags = msgFlags
		authProto, err := snmpAuthProtocol(auth.AuthProtocol)
		if err != nil {
			return nil, err
		}
		if authProto == gosnmp.NoAuth && (msgFlags == gosnmp.AuthNoPriv || msgFlags == gosnmp.AuthPriv) {
			authProto = gosnmp.SHA
		}
		privProto, err := snmpPrivProtocol(auth.PrivProtocol)
		if err != nil {
			return nil, err
		}
		if privProto == gosnmp.NoPriv && msgFlags == gosnmp.AuthPriv {
			privProto = gosnmp.AES
		}
		g.SecurityParameters = &gosnmp.UsmSecurityParameters{
			UserName:                 auth.Username,
			AuthenticationProtocol:   authProto,
			AuthenticationPassphrase: auth.AuthPassword,
			PrivacyProtocol:          privProto,
			PrivacyPassphrase:        auth.PrivPassword,
		}
	default:
		return nil, fmt.Errorf("unsupported snmp version %q", auth.Version)
	}
	return g, nil
}

func snmpMsgFlags(level string) (gosnmp.SnmpV3MsgFlags, error) {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "", "noauthnopriv":
		return gosnmp.NoAuthNoPriv, nil
	case "authnopriv":
		return gosnmp.AuthNoPriv, nil
	case "authpriv":
		return gosnmp.AuthPriv, nil
	default:
		return 0, fmt.Errorf("invalid security_level %q", level)
	}
}

func snmpAuthProtocol(p string) (gosnmp.SnmpV3AuthProtocol, error) {
	switch strings.ToUpper(strings.TrimSpace(p)) {
	case "":
		return gosnmp.NoAuth, nil
	case "MD5":
		return gosnmp.MD5, nil
	case "SHA":
		return gosnmp.SHA, nil
	case "SHA224":
		return gosnmp.SHA224, nil
	case "SHA256":
		return gosnmp.SHA256, nil
	case "SHA384":
		return gosnmp.SHA384, nil
	case "SHA512":
		return gosnmp.SHA512, nil
	default:
		return 0, fmt.Errorf("invalid auth_protocol %q", p)
	}
}

func snmpPrivProtocol(p string) (gosnmp.SnmpV3PrivProtocol, error) {
	switch strings.ToUpper(strings.TrimSpace(p)) {
	case "":
		return gosnmp.NoPriv, nil
	case "DES":
		return gosnmp.DES, nil
	case "AES", "AES128":
		return gosnmp.AES, nil
	case "AES192":
		return gosnmp.AES192, nil
	case "AES256":
		return gosnmp.AES256, nil
	default:
		return 0, fmt.Errorf("invalid priv_protocol %q", p)
	}
}
