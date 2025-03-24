package botclient

import (
	"context"

	"github.com/companyzero/bisonrelay/clientrpc/jsonrpc"
	"github.com/companyzero/bisonrelay/clientrpc/types"
	"github.com/decred/slog"
	"github.com/vctt94/bisonbotkit/config"
	"github.com/vctt94/bisonbotkit/logging"
)

// Client represents a BisonRelay client connection
type BotClient struct {
	RPCClient  *jsonrpc.WSClient
	Config     *config.ClientConfig
	LogBackend *logging.LogBackend
	Logger     slog.Logger
	Chat       types.ChatServiceClient
	Payment    types.PaymentsServiceClient
}

// NewClient creates a new BisonRelay client
func NewClient(cfg *config.ClientConfig, logBackend *logging.LogBackend) (*BotClient, error) {
	// Get logger from the backend
	log := logBackend.Logger("client")

	// Create JSON-RPC client
	rpcClient, err := jsonrpc.NewWSClient(
		jsonrpc.WithWebsocketURL(cfg.RPCURL),
		jsonrpc.WithServerTLSCertPath(cfg.ServerCertPath),
		jsonrpc.WithClientTLSCert(cfg.ClientCertPath, cfg.ClientKeyPath),
		jsonrpc.WithClientLog(log),
		jsonrpc.WithClientBasicAuth(cfg.RPCUser, cfg.RPCPass),
	)
	if err != nil {
		return nil, err
	}

	return &BotClient{
		RPCClient:  rpcClient,
		Config:     cfg,
		LogBackend: logBackend,
		Logger:     log,
		Chat:       types.NewChatServiceClient(rpcClient),
		Payment:    types.NewPaymentsServiceClient(rpcClient),
	}, nil
}

// RunRPC runs the BisonRelay rpc client service
func (c *BotClient) RunRPC(ctx context.Context) error {
	return c.RPCClient.Run(ctx)
}

// Stop closes the connection to the BisonRelay service
func (c *BotClient) StopRPC() error {
	return c.RPCClient.Close()
}
