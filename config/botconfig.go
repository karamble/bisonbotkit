package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vctt94/bisonbotkit/utils"
)

var (
	defaultBRClientDir = utils.AppDataDir("brclient", false)
)

// BotConfig holds all configuration options for a Bison Relay bot
type BotConfig struct {
	DataDir        string
	IsF2P          bool
	MinBetAmt      float64
	RPCURL         string
	GRPCHost       string
	GRPCPort       string
	HttpPort       string
	ServerCertPath string
	ClientCertPath string
	ClientKeyPath  string
	RPCUser        string
	RPCPass        string
	Debug          string
	// Logging-related fields
	LogFile        string // Path to the log file
	MaxLogFiles    int    // Maximum number of log files to keep
	MaxBufferLines int    // Maximum number of log lines to buffer
}

// Write the configuration to a file.
func writeConfigFile(cfg *BotConfig, configPath string) error {
	configData := fmt.Sprintf(
		`datadir=%s
isf2p=%t
minbetamt=%0.8f
rpcurl=%s
grpchost=%s
grpcport=%s
httpport=%s
servercertpath=%s
clientcertpath=%s
clientkeypath=%s
rpcuser=%s
rpcpass=%s
debug=%s
logfile=%s
maxlogfiles=%d
maxbufferlines=%d
`,
		cfg.DataDir,
		cfg.IsF2P,
		cfg.MinBetAmt,
		cfg.RPCURL,
		cfg.GRPCHost,
		cfg.GRPCPort,
		cfg.HttpPort,
		cfg.ServerCertPath,
		cfg.ClientCertPath,
		cfg.ClientKeyPath,
		cfg.RPCUser,
		cfg.RPCPass,
		cfg.Debug,
		cfg.LogFile,
		cfg.MaxLogFiles,
		cfg.MaxBufferLines,
	)

	return os.WriteFile(configPath, []byte(configData), 0600)
}

// parseConfigFile parses the config file at the given path into a BotConfig struct.
func parseConfigFile(configPath string) (*BotConfig, error) {
	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	cfg := &BotConfig{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "datadir":
			cfg.DataDir = value
		case "isf2p":
			cfg.IsF2P = value == "true"
		case "minbetamt":
			fmt.Sscanf(value, "%f", &cfg.MinBetAmt)
		case "rpcurl":
			cfg.RPCURL = value
		case "grpchost":
			cfg.GRPCHost = value
		case "grpcport":
			cfg.GRPCPort = value
		case "httpport":
			cfg.HttpPort = value
		case "servercertpath":
			cfg.ServerCertPath = value
		case "clientcertpath":
			cfg.ClientCertPath = value
		case "clientkeypath":
			cfg.ClientKeyPath = value
		case "rpcuser":
			cfg.RPCUser = value
		case "rpcpass":
			cfg.RPCPass = value
		case "debug":
			cfg.Debug = value
		case "logfile":
			cfg.LogFile = value
		case "maxlogfiles":
			fmt.Sscanf(value, "%d", &cfg.MaxLogFiles)
		case "maxbufferlines":
			fmt.Sscanf(value, "%d", &cfg.MaxBufferLines)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// LoadBotConfig attempts to load the bot config from the default locations.
func LoadBotConfig(configPath string, fileName string) (*BotConfig, error) {
	defaultConfigPath := utils.AppDataDir(fileName, false)
	configPath = utils.CleanAndExpandPath(configPath)
	// If configPath is empty, use defaultConfigPath
	if configPath == "" {
		configPath = defaultConfigPath
	}

	// Ensure the config directory exists
	if err := os.MkdirAll(configPath, 0700); err != nil {
		return nil, err
	}

	fullPath := filepath.Join(configPath, fileName)
	if _, err := os.Stat(fullPath); err == nil {
		cfg, err := parseConfigFile(fullPath)
		if err == nil {
			return cfg, nil
		}
	}

	// If we get here, either the file doesn't exist or couldn't be parsed
	// Generate new credentials and create default config
	rpcUser, err := utils.GenerateRandomString(8)
	if err != nil {
		return nil, err
	}
	rpcPass, err := utils.GenerateRandomString(16)
	if err != nil {
		return nil, err
	}

	cfg := &BotConfig{
		DataDir:        configPath,
		RPCURL:         "wss://127.0.0.1:7676/ws",
		GRPCHost:       "127.0.0.1",
		GRPCPort:       "50051",
		HttpPort:       "8888",
		ServerCertPath: filepath.Join(defaultBRClientDir, "rpc.cert"),
		ClientCertPath: filepath.Join(defaultBRClientDir, "rpc-client.cert"),
		ClientKeyPath:  filepath.Join(defaultBRClientDir, "rpc-client.key"),
		RPCUser:        rpcUser,
		RPCPass:        rpcPass,
		Debug:          "info",
		MinBetAmt:      0.00000001,
		LogFile:        filepath.Join(configPath, "logs", "chatbot.log"),
		MaxLogFiles:    5,
		MaxBufferLines: 1000,
	}

	// Write default config
	if err := writeConfigFile(cfg, fullPath); err != nil {
		return nil, fmt.Errorf("failed to write config file: %v", err)
	}

	return cfg, nil
}
