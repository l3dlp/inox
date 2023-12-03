package net_ns

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/miekg/dns"
)

const (
	HTTP_UPLOAD_RATE_LIMIT_NAME     = "http/upload"
	WS_SIMUL_CONN_TOTAL_LIMIT_NAME  = "ws/simul-connection"
	TCP_SIMUL_CONN_TOTAL_LIMIT_NAME = "tcp/simul-connection"
	HTTP_REQUEST_RATE_LIMIT_NAME    = "http/request"

	DEFAULT_TCP_DIAL_TIMEOUT        = 10 * time.Second
	DEFAULT_TCP_KEEP_ALIVE_INTERVAL = 10 * time.Second
	DEFAULT_TCP_BUFF_SIZE           = 1 << 16

	DEFAULT_HTTP_CLIENT_TIMEOUT = 10 * time.Second

	OPTION_DOES_NOT_EXIST_FMT = "option '%s' does not exist"
)

func websocketConnect(ctx *core.Context, u core.URL, options ...core.Option) (*WebsocketConnection, error) {
	insecure := false

	for _, opt := range options {
		switch opt.Name {
		case "insecure":
			insecure = bool(opt.Value.(core.Bool))
		default:
			return nil, commonfmt.FmtErrInvalidOptionName(opt.Name)
		}
	}

	return WebsocketConnect(WebsocketConnectParams{
		Ctx:      ctx,
		URL:      u,
		Insecure: insecure,
	})
}

type WebsocketConnectParams struct {
	Ctx              *core.Context
	URL              core.URL
	Insecure         bool
	RequestHeader    http.Header
	MessageTimeout   time.Duration //if 0 defaults to DEFAULT_WS_MESSAGE_TIMEOUT
	HandshakeTimeout time.Duration //if 0 defaults to DEFAULT_WS_HANDSHAKE_TIMEOUT
}

func WebsocketConnect(args WebsocketConnectParams) (*WebsocketConnection, error) {
	ctx := args.Ctx
	u := args.URL
	insecure := args.Insecure
	requestHeader := args.RequestHeader
	messageTimeout := utils.DefaultIfZero(args.MessageTimeout, DEFAULT_WS_MESSAGE_TIMEOUT)
	handshakeTimeout := utils.DefaultIfZero(args.HandshakeTimeout, DEFAULT_WS_HANDSHAKE_TIMEOUT)

	//check that a websocket read or write-stream permission is granted
	perm := core.WebsocketPermission{
		Kind_:    permkind.WriteStream,
		Endpoint: u,
	}

	if err := ctx.CheckHasPermission(perm); err != nil {
		perm.Kind_ = permkind.Read

		if err := ctx.CheckHasPermission(perm); err != nil {
			return nil, err
		}
	}

	ctx.Take(WS_SIMUL_CONN_TOTAL_LIMIT_NAME, 1)

	dialer := *websocket.DefaultDialer
	dialer.HandshakeTimeout = handshakeTimeout
	dialer.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: insecure,
	}

	c, resp, err := dialer.Dial(string(u), requestHeader)
	if err != nil {
		ctx.GiveBack(WS_SIMUL_CONN_TOTAL_LIMIT_NAME, 1)

		if resp == nil {
			return nil, fmt.Errorf("dial: %s", err.Error())
		} else {
			return nil, fmt.Errorf("dial: %s (http status code: %d, text: %s)", err.Error(), resp.StatusCode, resp.Status)
		}
	}

	return &WebsocketConnection{
		conn:           c,
		endpoint:       u,
		messageTimeout: messageTimeout,
		serverContext:  ctx,
	}, nil
}

func dnsResolve(ctx *core.Context, domain core.Str, recordTypeName core.Str) ([]core.Str, error) {
	defaultConfig, _ := dns.ClientConfigFromFile("/etc/resolv.conf")
	client := new(dns.Client)
	//TODO: reuse client ?

	msg := new(dns.Msg)
	var recordType uint16

	perm := core.DNSPermission{Kind_: permkind.Read, Domain: core.Host("://" + domain)}
	if err := ctx.CheckHasPermission(perm); err != nil {
		return nil, err
	}

	switch recordTypeName {
	case "A":
		recordType = dns.TypeA
	case "AAAA":
		recordType = dns.TypeAAAA
	case "CNAME":
		recordType = dns.TypeCNAME
	case "MX":
		recordType = dns.TypeMX
	default:
		return nil, fmt.Errorf("invalid DNS record type: '%s'", recordTypeName)
	}

	msg.SetQuestion(dns.Fqdn(string(domain)), recordType)
	msg.RecursionDesired = true

	r, _, err := client.Exchange(msg, net.JoinHostPort(defaultConfig.Servers[0], defaultConfig.Port))
	if r == nil {
		return nil, fmt.Errorf("dns: error: %s", err.Error())
	}

	if r.Rcode != dns.RcodeSuccess {
		return nil, fmt.Errorf("dns: failure: response code is %d", r.Rcode)
	}

	records := []core.Str{}
	for _, rr := range r.Answer {
		records = append(records, core.Str(rr.String()))
	}

	return records, nil
}

func tcpConnect(ctx *core.Context, host core.Host) (*TcpConn, error) {

	perm := core.RawTcpPermission{
		Kind_:  permkind.Read,
		Domain: host,
	}

	if err := ctx.CheckHasPermission(perm); err != nil {
		return nil, err
	}

	ctx.Take(TCP_SIMUL_CONN_TOTAL_LIMIT_NAME, 1)

	addr, err := net.ResolveTCPAddr("tcp", host.WithoutScheme())
	if err != nil {
		ctx.GiveBack(TCP_SIMUL_CONN_TOTAL_LIMIT_NAME, 1)
		return nil, err
	}

	dialer := net.Dialer{
		Timeout:   DEFAULT_TCP_DIAL_TIMEOUT,
		KeepAlive: DEFAULT_TCP_KEEP_ALIVE_INTERVAL,
	}

	conn, err := dialer.DialContext(ctx, "tcp", addr.String())
	if err != nil {
		ctx.GiveBack(TCP_SIMUL_CONN_TOTAL_LIMIT_NAME, 1)
		return nil, err
	}

	return &TcpConn{
		initialCtx: ctx,
		conn:       conn.(*net.TCPConn),
		host:       host,
	}, nil
}
