// Package bisonbotkit provides utilities for creating and managing BisonRelay bots.
package bisonbotkit

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/companyzero/bisonrelay/clientrpc/jsonrpc"
	"github.com/companyzero/bisonrelay/clientrpc/types"
	"github.com/decred/slog"
	"github.com/vctt94/bisonbotkit/config"
	"github.com/vctt94/bisonbotkit/logging"
	"golang.org/x/sync/errgroup"
)

func (b *Bot) Run(ctx context.Context) error {
	g, gctx := errgroup.WithContext(ctx)

	if b.gcChan != nil {
		g.Go(func() error {
			return b.gcNtfns(gctx)
		})
	}

	if b.inviteChan != nil {
		g.Go(func() error {
			return b.inviteNtfns(gctx)
		})
	}

	if b.pmChan != nil {
		g.Go(func() error {
			return b.pmNtfns(gctx)
		})
	}

	if b.kxChan != nil {
		g.Go(func() error {
			return b.kxNtfns(gctx)
		})
	}

	if b.postChan != nil {
		g.Go(func() error {
			return b.postNtfns(gctx)
		})
	}

	if b.postStatusChan != nil {
		g.Go(func() error {
			return b.postStatusNtfns(gctx)
		})
	}

	if b.tipProgressChan != nil {
		g.Go(func() error {
			return b.tipProgress(gctx)
		})
	}

	if b.tipReceivedChan != nil {
		g.Go(func() error {
			return b.tipReceived(gctx)
		})
	}

	return g.Wait()
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
	wsc, err := NewJSONRPCClient(cfg, logBackend.Logger("RPC"))
	if err != nil {
		return nil, err
	}

	wl := make(map[string]int64)
	wlFile := filepath.Join(cfg.DataDir, "whitelist.json")
	wlBytes, err := os.ReadFile(wlFile)
	switch {
	case os.IsNotExist(err):
	case err != nil:
		return nil, err
	default:
		if err = json.Unmarshal(wlBytes, &wl); err != nil {
			return nil, err
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		// XXX - kill everything if websocket returns
		err := wsc.Run(ctx)
		brLog := logBackend.Logger("BR")
		brLog.Errorf("websocket run ended: %v", err)
		cancel()
	}()

	return &Bot{
		cfg: cfg,
		wsc: wsc,
		ctx: ctx,

		gcChan:     cfg.GCChan,
		gcLog:      cfg.GCLog,
		inviteChan: cfg.InviteChan,

		pmChan: cfg.PMChan,
		pmLog:  cfg.PMLog,

		tipProgressChan: cfg.TipProgressChan,
		tipProgressLog:  cfg.TipLog,

		tipReceivedLog:  cfg.TipReceivedLog,
		tipReceivedChan: cfg.TipReceivedChan,

		kxChan: cfg.KXChan,
		kxLog:  cfg.KXLog,

		postChan: cfg.PostChan,
		postLog:  cfg.PostLog,

		postStatusChan: cfg.PostStatusChan,
		postStatusLog:  cfg.PostStatusLog,

		wl:     wl,
		wlFile: wlFile,

		chatService:    types.NewChatServiceClient(wsc),
		gcService:      types.NewGCServiceClient(wsc),
		paymentService: types.NewPaymentsServiceClient(wsc),
		postService:    types.NewPostsServiceClient(wsc),
	}, nil
}
