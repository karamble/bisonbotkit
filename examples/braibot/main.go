package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/companyzero/bisonrelay/clientrpc/types"
	kit "github.com/vctt94/bisonbotkit"
	"github.com/vctt94/bisonbotkit/config"
	"github.com/vctt94/bisonbotkit/logging"
)

var (
	flagAppRoot  = flag.String("approot", "~/.braibot", "Path to application data directory")
	currentModel = "fast-sdxl" // Default model
	debug        = true        // Set to true for debugging
)

// Define a struct for the model details
type Model struct {
	Name        string  // Name of the model
	Description string  // Description of the model
	Price       float64 // Price per picture in USD
}

// Update availableModels to hold Model structs
var availableModels = []Model{
	{
		Name:        "fast-sdxl",
		Description: "Fast model for generating images quickly.",
		Price:       0.0022,
	},
	{
		Name:        "hidream-i1-full",
		Description: "High-quality model for detailed images.",
		Price:       0.0050,
	},
	{
		Name:        "hidream-i1-dev",
		Description: "Development version of the HiDream model.",
		Price:       0.0035,
	},
	{
		Name:        "hidream-i1-fast",
		Description: "Faster version of the HiDream model.",
		Price:       0.0028,
	},
	{
		Name:        "flux-pro/v1.1",
		Description: "Professional model for high-end image generation.",
		Price:       0.0070,
	},
	{
		Name:        "flux-pro/v1.1-ultra",
		Description: "Ultra version of the professional model.",
		Price:       0.0100,
	},
	{
		Name:        "flux/schnell",
		Description: "Quick model for rapid image generation.",
		Price:       0.0015,
	},
}

// Map to hold the current model for each command
var currentModels = map[string]string{
	"text2image": "fast-sdxl", // Default model for text2image
}

// Command represents a bot command
type Command struct {
	Name        string
	Description string
	Handler     func(ctx context.Context, bot *kit.Bot, cfg *config.BotConfig, nick string, args []string) error
}

// FalResponse represents the response from Fal.ai API
type FalResponse struct {
	Status        string `json:"status,omitempty"`
	RequestID     string `json:"request_id,omitempty"`
	ResponseURL   string `json:"response_url,omitempty"`
	StatusURL     string `json:"status_url,omitempty"`
	CancelURL     string `json:"cancel_url,omitempty"`
	QueuePosition int    `json:"queue_position,omitempty"`
	Logs          []struct {
		Message   string `json:"message"`
		Level     string `json:"level"`
		Source    string `json:"source"`
		Timestamp string `json:"timestamp"`
	} `json:"logs,omitempty"`
	Response struct {
		Images []struct {
			URL         string `json:"url"`
			Width       int    `json:"width"`
			Height      int    `json:"height"`
			ContentType string `json:"content_type"`
		} `json:"images"`
	} `json:"response,omitempty"`
}

// Available commands
var commands map[string]Command

func init() {
	commands = map[string]Command{
		"help": {
			Name:        "help",
			Description: "Shows this help message",
			Handler: func(ctx context.Context, bot *kit.Bot, cfg *config.BotConfig, nick string, args []string) error {
				helpMsg := "Available commands:\n"
				for _, cmd := range commands {
					helpMsg += fmt.Sprintf("!%s - %s\n", cmd.Name, cmd.Description)
				}
				return bot.SendPM(ctx, nick, helpMsg)
			},
		},
		"listmodels": {
			Name:        "listmodels",
			Description: "Lists all available models for the text2image command.",
			Handler: func(ctx context.Context, bot *kit.Bot, cfg *config.BotConfig, nick string, args []string) error {
				modelList := "Available models for text2image:\n"
				for _, model := range availableModels {
					modelList += fmt.Sprintf("- %s: %s (Price: $%.4f)\n", model.Name, model.Description, model.Price)
				}
				return bot.SendPM(ctx, nick, modelList)
			},
		},
		"setmodel": {
			Name:        "setmodel",
			Description: "Sets the model to use for specified commands. Usage: !setmodel [command] [modelname]",
			Handler: func(ctx context.Context, bot *kit.Bot, cfg *config.BotConfig, nick string, args []string) error {
				if len(args) < 2 {
					return bot.SendPM(ctx, nick, "Please specify a command and a model name. Usage: !setmodel [command] [modelname]")
				}
				commandName := args[0]
				modelName := args[1]

				// Check if the command is valid
				if _, exists := commands[commandName]; !exists {
					return bot.SendPM(ctx, nick, "Invalid command name. Use !listmodels to see available commands.")
				}

				// Check if the model is valid
				for _, model := range availableModels {
					if model.Name == modelName {
						currentModels[commandName] = model.Name
						return bot.SendPM(ctx, nick, fmt.Sprintf("Model for %s set to: %s", commandName, model.Name))
					}
				}
				return bot.SendPM(ctx, nick, "Invalid model name. Use !listmodels to see available models.")
			},
		},
		"text2image": {
			Name:        "text2image",
			Description: "Generates an image from text prompt. Usage: !text2image [prompt]",
			Handler: func(ctx context.Context, bot *kit.Bot, cfg *config.BotConfig, nick string, args []string) error {
				if len(args) == 0 {
					return bot.SendPM(ctx, nick, "Please provide a prompt. Usage: !text2image [prompt]")
				}

				prompt := strings.Join(args, " ")

				// Prepare the request
				requestBody, err := json.Marshal(map[string]interface{}{
					"prompt": prompt,
				})
				if err != nil {
					return err
				}

				// Use the current model for text2image
				modelToUse := currentModels["text2image"]

				// Create HTTP request for initial call
				req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("https://queue.fal.run/fal-ai/%s", modelToUse), bytes.NewBuffer(requestBody))
				if err != nil {
					return err
				}

				// Set headers
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Key "+cfg.ExtraConfig["falapikey"])

				// Send request
				client := &http.Client{}
				resp, err := client.Do(req)
				if err != nil {
					return err
				}
				defer resp.Body.Close()

				// Read response
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return err
				}

				// Parse initial response
				var initialResp FalResponse
				if err := json.Unmarshal(body, &initialResp); err != nil {
					return err
				}

				// Poll until completion
				ticker := time.NewTicker(500 * time.Millisecond)
				defer ticker.Stop()

				for {
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-ticker.C:
						// Check status with logs enabled
						statusReq, err := http.NewRequestWithContext(ctx, "GET", initialResp.StatusURL+"?logs=1", nil)
						if err != nil {
							return err
						}
						statusReq.Header.Set("Authorization", "Key "+cfg.ExtraConfig["falapikey"])

						statusResp, err := client.Do(statusReq)
						if err != nil {
							return err
						}

						statusBody, err := io.ReadAll(statusResp.Body)
						statusResp.Body.Close()
						if err != nil {
							return err
						}

						var statusResponse FalResponse
						if err := json.Unmarshal(statusBody, &statusResponse); err != nil {
							return err
						}

						switch statusResponse.Status {
						case "IN_QUEUE":
							// Send queue position update
							bot.SendPM(ctx, nick, fmt.Sprintf("Your request is in queue. Position: %d", statusResponse.QueuePosition))
							continue
						case "IN_PROGRESS":
							// Log progress if available
							if len(statusResponse.Logs) > 0 {
								bot.SendPM(ctx, nick, fmt.Sprintf("Processing: %s", statusResponse.Logs[len(statusResponse.Logs)-1].Message))
							}
							continue
						case "COMPLETED":
							// Fetch final response
							finalReq, err := http.NewRequestWithContext(ctx, "GET", initialResp.ResponseURL, nil)
							if err != nil {
								return err
							}
							finalReq.Header.Set("Authorization", "Key "+cfg.ExtraConfig["falapikey"])

							finalResp, err := client.Do(finalReq)
							if err != nil {
								return err
							}
							defer finalResp.Body.Close()

							// Check the status code
							if finalResp.StatusCode != http.StatusOK {
								body, _ := io.ReadAll(finalResp.Body) // Read the body for logging
								return bot.SendPM(ctx, nick, fmt.Sprintf("Error fetching final response: %s. Body: %s", finalResp.Status, string(body)))
							}

							finalBody, err := io.ReadAll(finalResp.Body)
							if err != nil {
								return err
							}

							// Debug output
							if debug {
								fmt.Printf("Final Response Body: %s\n", string(finalBody))
							}

							// Unmarshal the final response
							var finalResponse struct {
								Images []struct {
									URL         string `json:"url"`
									Width       int    `json:"width"`
									Height      int    `json:"height"`
									ContentType string `json:"content_type"`
								} `json:"images"`
								Timings struct {
									Inference float64 `json:"inference"`
								} `json:"timings"`
								Seed            json.Number `json:"seed"`
								HasNSFWConcepts []bool      `json:"has_nsfw_concepts"`
								Prompt          string      `json:"prompt"`
							}
							if err := json.Unmarshal(finalBody, &finalResponse); err != nil {
								return err
							}

							// Assuming the first image is the one we want to send
							if len(finalResponse.Images) > 0 {
								imageURL := finalResponse.Images[0].URL
								// Fetch the image data
								imgResp, err := http.Get(imageURL)
								if err != nil {
									return err
								}
								defer imgResp.Body.Close()

								imgData, err := io.ReadAll(imgResp.Body)
								if err != nil {
									return err
								}

								// Encode the image data to base64
								encodedImage := base64.StdEncoding.EncodeToString(imgData)

								// Determine the image type from ContentType
								var imageType string
								switch finalResponse.Images[0].ContentType {
								case "image/jpeg":
									imageType = "image/jpeg"
								case "image/png":
									imageType = "image/png"
								case "image/webp":
									imageType = "image/webp"
								default:
									imageType = "image/jpeg" // Fallback to jpeg if unknown
								}

								// Create the message with embedded image, using the user's prompt as the alt text
								message := fmt.Sprintf("--embed[alt=%s,type=%s,data=%s]--", url.QueryEscape(prompt), imageType, encodedImage)
								return bot.SendPM(ctx, nick, message)
							} else {
								return bot.SendPM(ctx, nick, "No images were generated.")
							}
						case "FAILED":
							// Send the complete raw response body as PM
							responseMessage := fmt.Sprintf("Failed to generate image. Complete response: %s", string(statusBody))
							return bot.SendPM(ctx, nick, responseMessage)
						default:
							// Still processing, continue polling
							continue
						}
					}
				}
			},
		},
	}
}

// isCommand checks if a message is a command (starts with !)
func isCommand(msg string) (string, []string, bool) {
	if !strings.HasPrefix(msg, "!") {
		return "", nil, false
	}

	parts := strings.Fields(msg[1:]) // Remove ! and split
	if len(parts) == 0 {
		return "", nil, false
	}

	cmd := strings.ToLower(parts[0])
	args := parts[1:]
	return cmd, args, true
}

func realMain() error {
	flag.Parse()

	// Initialize logging
	logBackend, err := logging.NewLogBackend(logging.LogConfig{
		LogFile:        filepath.Join(*flagAppRoot, "logs", "braibot.log"),
		DebugLevel:     "info",
		MaxLogFiles:    5,
		MaxBufferLines: 1000,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize logging: %v", err)
	}
	defer logBackend.Close()

	// Get a logger for the application
	log := logBackend.Logger("BraiBot")

	// Load bot configuration
	cfg, err := config.LoadBotConfig(*flagAppRoot, "braibot.conf")
	if err != nil {
		return fmt.Errorf("failed to load config: %v", err)
	}

	// Create a bidirectional channel
	pmChan := make(chan types.ReceivedPM)
	// Assign the send side to the config
	cfg.PMChan = pmChan
	cfg.PMLog = logBackend.Logger("PM")

	// Create new bot instance
	bot, err := kit.NewBot(cfg, logBackend)
	if err != nil {
		return fmt.Errorf("failed to create bot: %v", err)
	}

	// Add a goroutine to handle PMs using our bidirectional channel
	go func() {
		for pm := range pmChan {
			log.Infof("Received PM from %s: %s", pm.Nick, pm.Msg.Message)

			// Check if the message is a command
			if cmd, args, isCmd := isCommand(pm.Msg.Message); isCmd {
				if command, exists := commands[cmd]; exists {
					if err := command.Handler(context.Background(), bot, cfg, pm.Nick, args); err != nil {
						log.Warnf("Error executing command %s: %v", cmd, err)
					}
				} else {
					// Send error message for unknown command
					bot.SendPM(context.Background(), pm.Nick, "Unknown command. Use !help to see available commands.")
				}
			}
		}
	}()

	// Set up context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Add input handling goroutine
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			tokens := strings.SplitN(line, " ", 2)
			if len(tokens) != 2 {
				log.Warn("Invalid format. Use: <nick> <message>")
				continue
			}

			nick, msg := tokens[0], tokens[1]
			if err := bot.SendPM(ctx, nick, msg); err != nil {
				log.Warnf("Failed to send PM: %v", err)
				continue
			}
			log.Infof("-> %s: %s", nick, msg)
		}
		if err := scanner.Err(); err != nil {
			log.Errorf("Error reading input: %v", err)
		}
	}()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		log.Infof("Received shutdown signal: %v", sig)
		bot.Close()
		cancel()
	}()

	// Run the bot with the cancellable context
	if err := bot.Run(ctx); err != nil {
		return fmt.Errorf("bot error: %v", err)
	}

	return nil
}

func main() {
	if err := realMain(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
