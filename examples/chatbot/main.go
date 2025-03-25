package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/companyzero/bisonrelay/clientrpc/types"
	kit "github.com/vctt94/bisonbotkit"
	"github.com/vctt94/bisonbotkit/config"
	"github.com/vctt94/bisonbotkit/logging"
	"golang.org/x/sync/errgroup"
)

var (
	flagAppRoot = flag.String("approot", "~/.chatbot", "Path to application data directory")
)

func sendLoop(ctx context.Context, bot *kit.Bot) error {
	r := bufio.NewScanner(os.Stdin)
	for r.Scan() {
		line := strings.TrimSpace(r.Text())
		tokens := strings.SplitN(line, " ", 2)
		if len(tokens) != 2 {
			bot.Log.Warn("Read line from stdin without 2 tokens")
			continue
		}

		user, msg := tokens[0], tokens[1]
		req := &types.PMRequest{
			User: user,
			Msg: &types.RMPrivateMessage{
				Message: msg,
			},
		}
		var res types.PMResponse
		err := bot.Chat.PM(ctx, req, &res)
		if err != nil {
			if err == context.Canceled {
				return err
			}
			bot.Log.Warnf("Unable to send last message: %v", err)
			continue
		}

		fmt.Printf("-> %v %v\n", user, msg)
	}
	return r.Err()
}

func receiveLoop(ctx context.Context, bot *kit.Bot) error {
	var ackRes types.AckResponse
	var ackReq types.AckRequest

	for {
		streamReq := types.PMStreamRequest{UnackedFrom: ackReq.SequenceId}
		stream, err := bot.Chat.PMStream(ctx, &streamReq)
		if err != nil {
			if err == context.Canceled {
				return err
			}
			bot.Log.Warnf("Error while obtaining PM stream: %v", err)
			continue
		}

		for {
			var pm types.ReceivedPM
			err := stream.Recv(&pm)
			if err != nil {
				if err == context.Canceled {
					return err
				}
				bot.Log.Warnf("Error while receiving stream: %v", err)
				break
			}

			fmt.Printf("<- %v %v\n", pm.Nick, pm.Msg.Message)

			// Ack the message
			ackReq.SequenceId = pm.SequenceId
			err = bot.Chat.AckReceivedPM(ctx, &ackReq, &ackRes)
			if err != nil {
				bot.Log.Warnf("Error while ack'ing received pm: %v", err)
				break
			}
		}
	}
}

func realMain() error {
	flag.Parse()

	// Initialize logging
	logBackend, err := logging.NewLogBackend(logging.LogConfig{
		LogFile:        filepath.Join(*flagAppRoot, "logs", "chatbot.log"),
		DebugLevel:     "info",
		MaxLogFiles:    5,
		MaxBufferLines: 1000,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize logging: %v", err)
	}

	// Load bot configuration
	cfg, err := config.LoadBotConfig(*flagAppRoot, "chatbot.conf")
	if err != nil {
		return fmt.Errorf("failed to load config: %v", err)
	}

	// Create new bot instance
	bot, err := kit.NewBot(cfg, logBackend)
	if err != nil {
		return fmt.Errorf("failed to create bot: %v", err)
	}

	// Set up context and error group
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	g, gctx := errgroup.WithContext(ctx)

	// Start the bot's RPC client
	g.Go(func() error { return bot.Run(gctx) })

	// Start send and receive loops
	g.Go(func() error { return sendLoop(gctx, bot) })
	g.Go(func() error { return receiveLoop(gctx, bot) })

	return g.Wait()
}

func main() {
	if err := realMain(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
