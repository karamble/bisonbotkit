package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/companyzero/bisonrelay/clientrpc/types"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/decred/dcrd/dcrutil/v4"
	kit "github.com/vctt94/bisonbotkit"
	"github.com/vctt94/bisonbotkit/config"
	"github.com/vctt94/bisonbotkit/logging"
)

type BetManager struct {
	sync.RWMutex
	currentGame *Bet
}

type Bet struct {
	TotalPool int64 // in matoms
	Bets      map[zkidentity.ShortID]UserBet
	Status    string // "open" or "closed"
	Number    int    // The randomly generated number
}

type UserBet struct {
	Amount int64
	Number int // The actual number the user bet on
}

func NewBetManager() *BetManager {
	return &BetManager{
		currentGame: &Bet{
			Bets:   make(map[zkidentity.ShortID]UserBet),
			Status: "open",
		},
	}
}

var (
	flagAppRoot = flag.String("approot", "~/.bettingbot", "Path to application data directory")
)

// Handle incoming PMs for betting commands
func (bm *BetManager) handlePM(ctx context.Context, bot *kit.Bot, pm *types.ReceivedPM) {
	tokens := strings.Fields(pm.Msg.Message)
	if len(tokens) == 0 {
		return
	}

	cmd := strings.ToLower(tokens[0])
	switch cmd {
	case "!bet":
		if len(tokens) != 3 {
			bot.SendPM(ctx, pm.Nick, "Usage: !bet <number> <amount>")
			return
		}

		number, err := strconv.Atoi(tokens[1])
		if err != nil {
			bot.SendPM(ctx, pm.Nick, "Invalid number. Please enter a valid integer")
			return
		}

		amt, err := strconv.ParseFloat(tokens[2], 64)
		if err != nil {
			bot.SendPM(ctx, pm.Nick, "Invalid amount")
			return
		}
		amount, err := dcrutil.NewAmount(amt)
		if err != nil {
			bot.SendPM(ctx, pm.Nick, "Invalid amount")
			return
		}

		bm.Lock()
		if bm.currentGame.Status != "open" {
			bm.Unlock()
			bot.SendPM(ctx, pm.Nick, "No active game right now")
			return
		}

		var uid zkidentity.ShortID
		uid.FromString(pm.String())

		bm.currentGame.Bets[uid] = UserBet{
			Amount: int64(amount),
			Number: number,
		}
		bm.currentGame.TotalPool += int64(amount)
		bm.Unlock()

		oddEven := "odd"
		if number%2 == 0 {
			oddEven = "even"
		}
		bot.SendPM(ctx, pm.Nick, fmt.Sprintf("Please send %.8f DCR to confirm your bet on number %d (%s)", amount.ToCoin(), number, oddEven))

	case "!draw":
		bm.Lock()
		if bm.currentGame.Status != "open" || len(bm.currentGame.Bets) == 0 {
			bm.Unlock()
			bot.SendPM(ctx, pm.Nick, "No active game with bets")
			return
		}

		// Generate random number
		bm.currentGame.Status = "closed"
		bm.currentGame.Number = rand.Intn(100) + 1 // Random number between 1 and 100
		drawnOddEven := "odd"
		if bm.currentGame.Number%2 == 0 {
			drawnOddEven = "even"
		}

		// Announce result
		bot.SendPM(ctx, pm.Nick, fmt.Sprintf("Number drawn: %d (%s)", bm.currentGame.Number, drawnOddEven))

		// Process payouts
		processPayout(ctx, bot, bm.currentGame)

		// Start new game
		bm.currentGame = &Bet{
			Bets:   make(map[zkidentity.ShortID]UserBet),
			Status: "open",
		}
		bm.Unlock()
	}
}

func processPayout(ctx context.Context, bot *kit.Bot, bet *Bet) {
	drawnIsEven := bet.Number%2 == 0

	winningPool := int64(0)
	losingPool := int64(0)

	// Calculate pools based on odd/even
	for _, userBet := range bet.Bets {
		userBetIsEven := userBet.Number%2 == 0
		if userBetIsEven == drawnIsEven {
			winningPool += userBet.Amount
		} else {
			losingPool += userBet.Amount
		}
	}

	// Process payouts
	for uid, userBet := range bet.Bets {
		userBetIsEven := userBet.Number%2 == 0
		if userBetIsEven == drawnIsEven {
			// Winners get their bet back plus proportional share of losing pool
			proportion := float64(userBet.Amount) / float64(winningPool)
			winnings := userBet.Amount + int64(float64(losingPool)*proportion)
			bot.PayTip(ctx, uid, dcrutil.Amount(winnings), 3)
			bot.SendPM(ctx, uid.String(), fmt.Sprintf("Congratulations! You bet %d and won %.8f DCR", userBet.Number, dcrutil.Amount(winnings).ToCoin()))
		} else {
			bot.SendPM(ctx, uid.String(), fmt.Sprintf("Sorry, you bet %d and lost this round!", userBet.Number))
		}
	}
}

// Handle incoming tips to confirm bets
func (bm *BetManager) handleTip(ctx context.Context, bot *kit.Bot, tip *types.ReceivedTip) {
	bm.Lock()
	defer bm.Unlock()

	var userID zkidentity.ShortID
	userID.FromBytes(tip.Uid)

	// Check if user has a pending bet
	userBet, exists := bm.currentGame.Bets[userID]
	if !exists || bm.currentGame.Status != "open" {
		// No pending bet, refund the tip
		bot.PayTip(ctx, userID, dcrutil.Amount(tip.AmountMatoms), 3)
		bot.SendPM(ctx, userID.String(), "No pending bet found. Your tip has been refunded.")
		return
	}

	// Verify tip amount matches bet amount
	if tip.AmountMatoms != userBet.Amount {
		// Amount mismatch, refund the tip
		bot.PayTip(ctx, userID, dcrutil.Amount(tip.AmountMatoms), 3)
		bot.SendPM(ctx, userID.String(), "Tip amount doesn't match bet amount. Your tip has been refunded.")
		return
	}

	// Send confirmation
	oddEven := "odd"
	if userBet.Number%2 == 0 {
		oddEven = "even"
	}
	bot.SendPM(ctx, userID.String(), fmt.Sprintf("Bet confirmed! You bet %d (%s) for %.8f DCR",
		userBet.Number,
		oddEven,
		dcrutil.Amount(userBet.Amount).ToCoin()))
}

func realMain() error {
	flag.Parse()

	// Initialize logging
	logBackend, err := logging.NewLogBackend(logging.LogConfig{
		LogFile:     filepath.Join(*flagAppRoot, "logs", "bettingbot.log"),
		DebugLevel:  "info",
		MaxLogFiles: 5,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize logging: %v", err)
	}
	defer logBackend.Close()

	log := logBackend.Logger("BettingBot")

	// Load bot configuration
	cfg, err := config.LoadBotConfig(*flagAppRoot, "bettingbot.conf")
	if err != nil {
		return fmt.Errorf("failed to load config: %v", err)
	}

	// Create channels for handling tips and PMs
	tipChan := make(chan types.ReceivedTip)
	pmChan := make(chan types.ReceivedPM)
	tipProgressChan := make(chan types.TipProgressEvent)

	cfg.TipProgressChan = tipProgressChan
	cfg.PMChan = pmChan
	cfg.TipReceivedChan = tipChan
	cfg.TipLog = logBackend.Logger("TIP")
	cfg.PMLog = logBackend.Logger("PM")

	// Create new bot instance
	bot, err := kit.NewBot(cfg, logBackend)
	if err != nil {
		return fmt.Errorf("failed to create bot: %v", err)
	}

	// Set up context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		log.Infof("Received shutdown signal: %v", sig)
		bot.Close()
		cancel()
	}()

	betManager := NewBetManager()

	// Handle PMs
	go func() {
		for pm := range pmChan {
			betManager.handlePM(ctx, bot, &pm)
		}
	}()

	// Handle tips
	go func() {
		for tip := range tipChan {
			betManager.handleTip(ctx, bot, &tip)
		}
	}()

	return bot.Run(ctx)
}

func main() {
	if err := realMain(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
