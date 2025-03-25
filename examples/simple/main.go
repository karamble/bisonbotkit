package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/vctt94/bisonbotkit"
	"github.com/vctt94/bisonbotkit/config"
	"github.com/vctt94/bisonbotkit/logging"
	"github.com/vctt94/bisonbotkit/utils"
)

func main() {
	// Get application data directory
	appdata := utils.AppDataDir("simplebot", false)

	// Load bot configuration
	cfg, err := config.LoadBotConfig(appdata, "simplebot.conf")
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up logging
	logBackend, err := logging.NewLogBackend(logging.LogConfig{
		LogFile:        filepath.Join(appdata, "logs", "simplebot.log"),
		DebugLevel:     cfg.Debug,
		MaxLogFiles:    5,
		MaxBufferLines: 1000,
	})
	if err != nil {
		fmt.Printf("Failed to create log backend: %v\n", err)
		os.Exit(1)
	}
	defer logBackend.Close()

	// Get a logger for your application
	log := logBackend.Logger("SimpleBot")

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Infof("Received shutdown signal")
		cancel()
	}()

	// Create and run the bot
	bot, err := bisonbotkit.NewBot(cfg, logBackend)
	if err != nil {
		log.Errorf("Failed to create bot: %v", err)
		os.Exit(1)
	}

	// Run the bot
	if err := bot.Run(ctx); err != nil {
		log.Errorf("Bot error: %v", err)
		os.Exit(1)
	}
}
