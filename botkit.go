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

type Bot struct {
	Cfg       *config.BotConfig
	RPCClient *jsonrpc.WSClient
	Chat      types.ChatServiceClient
	Payment   types.PaymentsServiceClient
	Log       slog.Logger
}

func (b *Bot) Run(ctx context.Context) error {
	if b.RPCClient == nil {
		return fmt.Errorf("RPC client not initialized")
	}
	return b.RPCClient.Run(ctx)
}

// NewJSONRPCClient creates a new JSON-RPC client for BisonRelay
func NewJSONRPCClient(cfg *config.BotConfig, log slog.Logger) (*jsonrpc.WSClient, error) {
	return jsonrpc.NewWSClient(
		jsonrpc.WithWebsocketURL(cfg.RPCURL),
		jsonrpc.WithServerTLSCertPath(cfg.ServerCertPath),
		jsonrpc.WithClientTLSCert(cfg.ClientCertPath, cfg.ClientKeyPath),
		jsonrpc.WithClientLog(log),
		jsonrpc.WithClientBasicAuth(cfg.RPCUser, cfg.RPCPass),
	)
}

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
