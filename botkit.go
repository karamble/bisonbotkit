// Package bisonbotkit provides utilities for creating and managing BisonRelay bots.
package bisonbotkit

import (
	"context"
	"fmt"

	"github.com/companyzero/bisonrelay/clientrpc/jsonrpc"
	"github.com/companyzero/bisonrelay/clientrpc/types"
	"github.com/decred/slog"
	"github.com/vctt94/bisonbotkit/config"
	"github.com/vctt94/bisonbotkit/logging"
)

// Bot represents a BisonRelay bot instance with configuration, RPC clients,
// and service interfaces for chat and payments.
type Bot struct {
	// Cfg holds the bot's configuration settings
	Cfg *config.BotConfig
	// RPCClient is the WebSocket client for JSON-RPC communication
	RPCClient *jsonrpc.WSClient

	// Chat provides access to BisonRelay chat functionality
	Chat types.ChatServiceClient
	// Payment provides access to BisonRelay payment functionality
	Payment types.PaymentsServiceClient
	// Log is the bot's logger instance
	Log slog.Logger
}

// Run starts the bot and blocks until the context is cancelled or an error occurs.
// It manages the lifecycle of the RPC client connection.
func (b *Bot) Run(ctx context.Context) error {
	if b.RPCClient == nil {
		return fmt.Errorf("RPC client not initialized")
	}
	return b.RPCClient.Run(ctx)
}

// NewJSONRPCClient creates a new JSON-RPC client for BisonRelay with the specified
// configuration and logger. It sets up TLS certificates, authentication, and other
// connection parameters.
func NewJSONRPCClient(cfg *config.BotConfig, log slog.Logger) (*jsonrpc.WSClient, error) {
	return jsonrpc.NewWSClient(
		jsonrpc.WithWebsocketURL(cfg.RPCURL),
		jsonrpc.WithServerTLSCertPath(cfg.ServerCertPath),
		jsonrpc.WithClientTLSCert(cfg.ClientCertPath, cfg.ClientKeyPath),
		jsonrpc.WithClientLog(log),
		jsonrpc.WithClientBasicAuth(cfg.RPCUser, cfg.RPCPass),
	)
}

// NewBot creates a new Bot instance with the provided configuration and logging backend.
// It initializes the RPC client and sets up chat and payment service clients.
// Returns an error if the RPC client initialization fails.
func NewBot(cfg *config.BotConfig, logBackend *logging.LogBackend) (*Bot, error) {
	rpcClient, err := NewJSONRPCClient(cfg, logBackend.Logger("RPC"))
	if err != nil {
		return nil, err
	}
	chatClient := types.NewChatServiceClient(rpcClient)
	paymentClient := types.NewPaymentsServiceClient(rpcClient)
	return &Bot{
		Cfg:       cfg,
		RPCClient: rpcClient,
		Chat:      chatClient,
		Payment:   paymentClient,
	}, nil
}
