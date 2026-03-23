package cli

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	pkgutils "example.com/rbmq-demo/pkg/utils"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Please note, sensitive data such as token are provided via env, not presented in the command line.
type BotCmd struct {
	ListenAddress            string        `help:"Address to listen on." type:"string" default:":8083"`
	PublicEndpoint           string        `help:"Public endpoint of the bot." type:"string"`
	JWTAuthSecretFromEnv     string        `name:"jwt-auth-secret-from-env" help:"Name of the environment variable that contains the JWT secret" default:"JWT_SECRET"`
	JWTAuthSecretFromFile    string        `name:"jwt-auth-secret-from-file" help:"Path to the file that contains the JWT secret"`
	JWTIssuerName            string        `name:"jwt-issuer-name" help:"The issuer appeared in the signed jwt token" default:"globalping-hub"`
	TelegramWebhookSecretEnv string        `name:"tg-webhook-secret-env" help:"Name of the environment variable that stores the Telegram webhook secret" default:"TG_WS_SECRET"`
	TelegramBotSecretEnv     string        `name:"tg-bot-secret-env" help:"Name of the environment variable that stores the telegram bot secret" default:"TG_BOT_TOKEN"`
	TextStreamInterval       time.Duration `name:"tg-bot-text-stream-interval" help:"Sleeping interval between two consecutive Telegram bot text edit" default:"1500ms"`
	ButtonLayoutColumns      int           `name:"tg-bot-button-layout-columns" help:"Specify the number of columns of the layout of buttons grid of the bot's response message" default:"4"`
}

func (botCmd *BotCmd) getJWTSecret() ([]byte, error) {
	return getJWTSecFromSomewhere(botCmd.JWTAuthSecretFromEnv, botCmd.JWTAuthSecretFromFile)
}

type CtxKey string

const (
	CtxKeyJWTSecret      = CtxKey("jwt_secret")
	CtxKeyIssuerName     = CtxKey("issuer_name")
	CtxKeyTxtStreamIntv  = CtxKey("txt_stream_intv")
	CtxKeyTGBtnLayoutCol = CtxKey("tg_btn_layout_col")
)

func (botCmd *BotCmd) getTGBotSecret() (string, error) {
	if botCmd.TelegramBotSecretEnv == "" {
		return "", errors.New("TelegramBotSecretEnv must not be empty")
	}
	botToken := os.Getenv(botCmd.TelegramBotSecretEnv)
	if botToken == "" {
		return "", fmt.Errorf("%s is not set", botCmd.TelegramBotSecretEnv)
	}
	return botToken, nil
}

func (botCmd *BotCmd) getTGWebhookSecret() (string, error) {
	if botCmd.TelegramWebhookSecretEnv == "" {
		return "", errors.New("TelegramWebhookSecretEnv must not be empty")
	}
	tgWebSocketSecret := os.Getenv(botCmd.TelegramWebhookSecretEnv)
	if tgWebSocketSecret == "" {
		return "", fmt.Errorf("%s is not set", botCmd.TelegramWebhookSecretEnv)
	}
	return tgWebSocketSecret, nil
}

func (botCmd *BotCmd) Run() error {
	// the parent command's Run() method already loaded the .env file,
	// so it's not needed to repeat that here.

	secret, err := botCmd.getJWTSecret()
	if err != nil {
		return fmt.Errorf("failed to get JWT secret: %v", err)
	}

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ctx = context.WithValue(ctx, CtxKeyJWTSecret, secret)

	botToken, err := botCmd.getTGBotSecret()
	if err != nil {
		return fmt.Errorf("failed to get Telegram bot secret: %v", err)
	}

	tgWebSocketSecret, err := botCmd.getTGWebhookSecret()
	if err != nil {
		return fmt.Errorf("failed to get Telegram webhook secret: %v", err)
	}

	if botCmd.PublicEndpoint == "" {
		return fmt.Errorf("public endpoint is not set")
	}

	opts := []bot.Option{
		bot.WithDefaultHandler(handleDefault),
		bot.WithWebhookSecretToken(tgWebSocketSecret),
	}

	b, _ := bot.New(botToken, opts...)

	ok, err := b.SetWebhook(ctx, &bot.SetWebhookParams{
		URL:         botCmd.PublicEndpoint,
		SecretToken: tgWebSocketSecret,
	})
	if err != nil {
		log.Fatalf("failed to set webhook: %v", err)
	}
	if !ok {
		log.Fatalf("failed to set webhook")
	}
	log.Printf("Webhook set successfully to %s", botCmd.PublicEndpoint)

	defer b.DeleteWebhook(ctx, &bot.DeleteWebhookParams{})

	startedAt := time.Now()
	ctx = context.WithValue(ctx, pkgutils.CtxKeyStartedAt, startedAt)
	ctx = context.WithValue(ctx, CtxKeyTxtStreamIntv, botCmd.TextStreamInterval)
	ctx = context.WithValue(ctx, CtxKeyIssuerName, botCmd.JWTIssuerName)
	ctx = context.WithValue(ctx, CtxKeyTGBtnLayoutCol, botCmd.ButtonLayoutColumns)

	b.RegisterHandlerRegexp(bot.HandlerTypeMessageText, regexp.MustCompile(`^/start`), handleStart)
	b.RegisterHandlerRegexp(bot.HandlerTypeMessageText, regexp.MustCompile(`^/ping`), handlePing)
	b.RegisterHandlerRegexp(bot.HandlerTypeMessageText, regexp.MustCompile(`^/uptime`), handleUptime)
	b.RegisterHandlerRegexp(bot.HandlerTypeMessageText, regexp.MustCompile(`^/token`), handleToken)
	b.RegisterHandlerRegexp(bot.HandlerTypeCallbackQueryData, regexp.MustCompile(`^ping_location_.+$`), handlePingQueryCallback)

	go b.StartWebhook(ctx)

	listener, err := net.Listen("tcp", botCmd.ListenAddress)
	if err != nil {
		return fmt.Errorf("failed to listen on address %s: %v", botCmd.ListenAddress, err)
	}
	log.Printf("Listening on address %s", listener.Addr())

	go func() {
		server := http.Server{
			Handler: b.WebhookHandler(),
		}
		if err := server.Serve(listener); err != nil {
			if !errors.Is(err, net.ErrClosed) {
				log.Fatalf("failed to serve: %v", err)
			}
			log.Println("Server exitted")
		}
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigs
	log.Printf("Received %s, shutting down ...", sig.String())
	cancel()

	return nil
}

func handleDefault(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message != nil {
		if update.Message.Chat.Type == models.ChatTypePrivate {
			// private message
			log.Printf("Received private message from private chat %+v: %s", update.Message.Chat.Username, update.Message.Text)
			_, err := b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   update.Message.Text,
			})
			if err != nil {
				log.Printf("failed to send message: %v", err)
			}

		} else if update.Message.Chat.Type == models.ChatTypeGroup || update.Message.Chat.Type == models.ChatTypeSupergroup {
			log.Printf("Received group message from group %+v: %s", update.Message.Chat.Title, update.Message.Text)
		}
	}
}

func handleStart(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message != nil {
		if update.Message.Chat.Type == models.ChatTypePrivate {
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   "Already started!",
			})
		}
	}
}

// Mock ping data (same as in handlePing)
type PingEvent struct {
	Seq          int
	RTTMs        int
	Peer         string
	PeerRDNS     string
	IPPacketSize int
	Timeout      bool
}

// String returns a formatted string representation of the ping event
func (e *PingEvent) String() string {
	// Handle timeout events
	if e.Timeout {
		return fmt.Sprintf("Request timeout for icmp_seq %d", e.Seq)
	}

	// Handle normal events
	if e.PeerRDNS != "" {
		return fmt.Sprintf("%d bytes from %s (%s): icmp_seq=%d ttl=64 time=%d ms",
			e.IPPacketSize, e.Peer, e.PeerRDNS, e.Seq, e.RTTMs)
	}
	return fmt.Sprintf("%d bytes from %s: icmp_seq=%d ttl=64 time=%d ms",
		e.IPPacketSize, e.Peer, e.Seq, e.RTTMs)
}

type PingEventsProvider interface {
	GetEventsByLocationCode(ctx context.Context, locationCode string) <-chan PingEvent
	GetAllLocations(ctx context.Context) []LocationDescriptor
}

type MockPingEventsProvider struct{}

func (provider *MockPingEventsProvider) GetEventsByLocationCode(ctx context.Context, code string) <-chan PingEvent {
	lcode := strings.ToLower(code)

	evsToEVChan := func(evs []PingEvent) <-chan PingEvent {
		evsChan := make(chan PingEvent, 0)
		go func(evs []PingEvent) {
			defer close(evsChan)
			for _, ev := range evs {
				evsChan <- ev
			}
		}(evs)
		return evsChan
	}

	if lcode == "hk-hkg1" {
		evs := []PingEvent{
			{Seq: 0, RTTMs: 12, Peer: "10.0.1.1", PeerRDNS: "server-a1.local", IPPacketSize: 64, Timeout: false},
			{Seq: 1, RTTMs: 15, Peer: "10.0.1.2", PeerRDNS: "server-a2.local", IPPacketSize: 64, Timeout: false},
			{Seq: 2, RTTMs: 11, Peer: "10.0.1.3", PeerRDNS: "server-a3.local", IPPacketSize: 64, Timeout: false},
			{Seq: 3, RTTMs: 0, Peer: "10.0.1.4", PeerRDNS: "server-a4.local", IPPacketSize: 64, Timeout: true},
			{Seq: 4, RTTMs: 14, Peer: "10.0.1.5", PeerRDNS: "", IPPacketSize: 64, Timeout: false},
			{Seq: 5, RTTMs: 20, Peer: "10.0.1.6", PeerRDNS: "server-a6.local", IPPacketSize: 64, Timeout: false},
			{Seq: 6, RTTMs: 16, Peer: "10.0.1.7", PeerRDNS: "", IPPacketSize: 64, Timeout: false},
			{Seq: 7, RTTMs: 13, Peer: "10.0.1.8", PeerRDNS: "server-a8.local", IPPacketSize: 64, Timeout: false},
			{Seq: 8, RTTMs: 0, Peer: "10.0.1.9", PeerRDNS: "server-a9.local", IPPacketSize: 64, Timeout: true},
			{Seq: 9, RTTMs: 19, Peer: "10.0.1.10", PeerRDNS: "", IPPacketSize: 64, Timeout: false},
		}
		return evsToEVChan(evs)
	} else if lcode == "us-lax1" {
		return evsToEVChan([]PingEvent{
			{Seq: 10, RTTMs: 65, Peer: "10.0.2.1", PeerRDNS: "node-b1.example.com", IPPacketSize: 64, Timeout: false},
			{Seq: 11, RTTMs: 72, Peer: "10.0.2.2", PeerRDNS: "node-b2.example.com", IPPacketSize: 64, Timeout: false},
			{Seq: 12, RTTMs: 58, Peer: "10.0.2.3", PeerRDNS: "", IPPacketSize: 64, Timeout: false},
			{Seq: 13, RTTMs: 0, Peer: "10.0.2.4", PeerRDNS: "node-b4.example.com", IPPacketSize: 64, Timeout: true},
			{Seq: 14, RTTMs: 94, Peer: "10.0.2.5", PeerRDNS: "node-b5.example.com", IPPacketSize: 64, Timeout: false},
			{Seq: 15, RTTMs: 76, Peer: "10.0.2.6", PeerRDNS: "", IPPacketSize: 64, Timeout: false},
			{Seq: 16, RTTMs: 0, Peer: "10.0.2.7", PeerRDNS: "node-b7.example.com", IPPacketSize: 64, Timeout: true},
			{Seq: 17, RTTMs: 85, Peer: "10.0.2.8", PeerRDNS: "node-b8.example.com", IPPacketSize: 64, Timeout: false},
			{Seq: 18, RTTMs: 68, Peer: "10.0.2.9", PeerRDNS: "", IPPacketSize: 64, Timeout: false},
			{Seq: 19, RTTMs: 103, Peer: "10.0.2.10", PeerRDNS: "node-b10.example.com", IPPacketSize: 64, Timeout: false},
		})
	} else if lcode == "jp-tyo1" {
		return evsToEVChan([]PingEvent{
			{Seq: 20, RTTMs: 145, Peer: "192.168.100.1", PeerRDNS: "host-c1.remote.net", IPPacketSize: 64, Timeout: false},
			{Seq: 21, RTTMs: 187, Peer: "192.168.100.2", PeerRDNS: "host-c2.remote.net", IPPacketSize: 64, Timeout: false},
			{Seq: 22, RTTMs: 0, Peer: "192.168.100.3", PeerRDNS: "", IPPacketSize: 64, Timeout: true},
			{Seq: 23, RTTMs: 203, Peer: "192.168.100.4", PeerRDNS: "host-c4.remote.net", IPPacketSize: 64, Timeout: false},
			{Seq: 24, RTTMs: 178, Peer: "192.168.100.5", PeerRDNS: "host-c5.remote.net", IPPacketSize: 64, Timeout: false},
			{Seq: 25, RTTMs: 134, Peer: "192.168.100.6", PeerRDNS: "", IPPacketSize: 64, Timeout: false},
			{Seq: 26, RTTMs: 0, Peer: "192.168.100.7", PeerRDNS: "host-c7.remote.net", IPPacketSize: 64, Timeout: true},
			{Seq: 27, RTTMs: 167, Peer: "192.168.100.8", PeerRDNS: "host-c8.remote.net", IPPacketSize: 64, Timeout: false},
			{Seq: 28, RTTMs: 198, Peer: "192.168.100.9", PeerRDNS: "", IPPacketSize: 64, Timeout: false},
			{Seq: 29, RTTMs: 0, Peer: "192.168.100.10", PeerRDNS: "host-c10.remote.net", IPPacketSize: 64, Timeout: true},
		})
	} else if lcode == "de-fra1" {
		return evsToEVChan([]PingEvent{
			{Seq: 30, RTTMs: 312, Peer: "172.16.50.1", PeerRDNS: "far-d1.distant.io", IPPacketSize: 64, Timeout: false},
			{Seq: 31, RTTMs: 0, Peer: "172.16.50.2", PeerRDNS: "far-d2.distant.io", IPPacketSize: 64, Timeout: true},
			{Seq: 32, RTTMs: 378, Peer: "172.16.50.3", PeerRDNS: "", IPPacketSize: 64, Timeout: false},
			{Seq: 33, RTTMs: 534, Peer: "172.16.50.4", PeerRDNS: "far-d4.distant.io", IPPacketSize: 64, Timeout: false},
			{Seq: 34, RTTMs: 0, Peer: "172.16.50.5", PeerRDNS: "far-d5.distant.io", IPPacketSize: 64, Timeout: true},
			{Seq: 35, RTTMs: 467, Peer: "172.16.50.6", PeerRDNS: "", IPPacketSize: 64, Timeout: false},
			{Seq: 36, RTTMs: 398, Peer: "172.16.50.7", PeerRDNS: "far-d7.distant.io", IPPacketSize: 64, Timeout: false},
			{Seq: 37, RTTMs: 0, Peer: "172.16.50.8", PeerRDNS: "far-d8.distant.io", IPPacketSize: 64, Timeout: true},
			{Seq: 38, RTTMs: 423, Peer: "172.16.50.9", PeerRDNS: "", IPPacketSize: 64, Timeout: false},
			{Seq: 39, RTTMs: 0, Peer: "172.16.50.10", PeerRDNS: "far-d10.distant.io", IPPacketSize: 64, Timeout: true},
		})
	} else {
		return evsToEVChan([]PingEvent{})
	}
}

type LocationDescriptor struct {
	Id                string
	Label             string
	Alpha2CountryCode string
	CityIATACode      string
}

func (provider *MockPingEventsProvider) GetAllLocations(ctx context.Context) []LocationDescriptor {
	return []LocationDescriptor{
		{Id: "hk-hkg1", Label: "HKG1", Alpha2CountryCode: "HK", CityIATACode: "HKG"},
		{Id: "us-lax1", Label: "LAX1", Alpha2CountryCode: "US", CityIATACode: "LAX"},
		{Id: "jp-tyo1", Label: "TYO1", Alpha2CountryCode: "JP", CityIATACode: "TYO"},
		{Id: "de-fra1", Label: "FRA1", Alpha2CountryCode: "DE", CityIATACode: "FRA"},
	}
}

// PingStatistics holds calculated statistics for a ping task
type PingStatistics struct {
	ReceivedPktCount int
	LossPktCount     int
	MinRTT           int
	MaxRTT           int
	AvgRTT           int
}

// String returns a formatted string representation of the ping statistics
func (s *PingStatistics) String() string {
	totalPkts := s.ReceivedPktCount + s.LossPktCount
	lossPercent := 0.0
	if totalPkts > 0 {
		lossPercent = float64(s.LossPktCount) / float64(totalPkts) * 100
	}
	return fmt.Sprintf("--- ping statistics ---\n"+
		"%d packets transmitted, %d packets received, %.1f%% packet loss\n"+
		"round-trip min/avg/max = %d/%d/%d ms",
		totalPkts, s.ReceivedPktCount, lossPercent, s.MinRTT, s.AvgRTT, s.MaxRTT)
}

type PingStatisticsBuilder struct {
	pingEvs          []PingEvent
	receivedPktCount int
	lossPktCount     int
	minRTT           int
	maxRTT           int
	totalRTT         int
}

func (statsBuilder *PingStatisticsBuilder) WriteEvent(ev PingEvent) {
	statsBuilder.pingEvs = append(statsBuilder.pingEvs, ev)

	// Update packet counts
	if ev.Timeout {
		statsBuilder.lossPktCount++
	} else {
		statsBuilder.receivedPktCount++
		// Update RTT statistics for non-timeout packets
		// For the first received packet, initialize min and max RTT
		if statsBuilder.receivedPktCount == 1 {
			statsBuilder.minRTT = ev.RTTMs
			statsBuilder.maxRTT = ev.RTTMs
		} else {
			if ev.RTTMs < statsBuilder.minRTT {
				statsBuilder.minRTT = ev.RTTMs
			}
			if ev.RTTMs > statsBuilder.maxRTT {
				statsBuilder.maxRTT = ev.RTTMs
			}
		}
		statsBuilder.totalRTT += ev.RTTMs
	}
}

// getPingStatistics calculates and returns statistics for a given ping task.
// Returns nil if no events found for the task.
func (statsBuilder *PingStatisticsBuilder) GetPingStatistics() *PingStatistics {
	if len(statsBuilder.pingEvs) == 0 {
		return nil
	}

	// Calculate average RTT
	avgRTT := 0
	if statsBuilder.receivedPktCount > 0 {
		avgRTT = statsBuilder.totalRTT / statsBuilder.receivedPktCount
	}

	return &PingStatistics{
		ReceivedPktCount: statsBuilder.receivedPktCount,
		LossPktCount:     statsBuilder.lossPktCount,
		MinRTT:           statsBuilder.minRTT,
		MaxRTT:           statsBuilder.maxRTT,
		AvgRTT:           avgRTT,
	}
}

// getFormattedPingEvents returns a formatted string of ping events for a given ping task,
// similar to the output of a ping command (individual replies, not statistics).
// Returns an empty string if no events found for the ping task.
func (statsBuilder *PingStatisticsBuilder) GetFormattedPingEvents() string {
	pingEvs := statsBuilder.pingEvs

	if len(pingEvs) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, event := range pingEvs {
		sb.WriteString(event.String() + "\n")
	}

	return sb.String()
}

func (statsBuilder *PingStatisticsBuilder) GetHumanReadableText() string {
	stats := ""
	if s := statsBuilder.GetPingStatistics(); s != nil {
		stats = s.String()
	}

	pingEvents := statsBuilder.GetFormattedPingEvents()
	txt := pingEvents + "\n" + stats
	return txt
}

func FormatPingCallbackData(locationCode string) string {
	return fmt.Sprintf("ping_location_%s", locationCode)
}

func ParseLocationCodeFromPingCallbackData(pingCallbackData string) string {
	if suffix, found := strings.CutPrefix(pingCallbackData, "ping_location_"); found {
		return suffix
	}
	return ""
}

// Takes an ISO 3166-1 alpha-2 code, returns a flag unicode string
func alpha2CountryCodeToUnicode(alpha2 string) string {
	if len(alpha2) != 2 {
		return ""
	}

	// Convert to uppercase
	alpha2 = strings.ToUpper(alpha2)

	// Get the two letters
	letter1 := rune(alpha2[0])
	letter2 := rune(alpha2[1])

	// Validate they are A-Z
	if letter1 < 'A' || letter1 > 'Z' || letter2 < 'A' || letter2 > 'Z' {
		return ""
	}

	// Calculate regional indicator symbols
	// Regional indicator A is at U+1F1E6, B is at U+1F1E7, etc.
	flag1 := '\U0001F1E6' + (letter1 - 'A')
	flag2 := '\U0001F1E6' + (letter2 - 'A')

	// Combine them to form the flag emoji
	return string(flag1) + string(flag2)
}

// getLocationButtonText returns the button text for a ping task, with a checkmark if selected.
func getLocationButtonText(loc LocationDescriptor, activeLocationCode string) string {
	activationMark := ""
	if loc.Id == activeLocationCode {
		activationMark = " ✓"
	}
	return alpha2CountryCodeToUnicode(loc.Alpha2CountryCode) + loc.Label + activationMark
}

// GetLocationButtons returns an inline keyboard markup with location buttons,
// showing a checkmark indicator on the currently selected location.
func GetLocationButtons(ctx context.Context, selectedLocationCode string, provider *MockPingEventsProvider, numCols int) *models.InlineKeyboardMarkup {
	allLocations := provider.GetAllLocations(ctx)
	buttons := make([][]models.InlineKeyboardButton, 0)

	// Create buttons and organize them into rows with numCols buttons each
	for i, loc := range allLocations {
		// Start a new row if we're at the beginning or at a column boundary
		if i%numCols == 0 {
			buttons = append(buttons, make([]models.InlineKeyboardButton, 0))
		}

		// Add button to the current row
		currentRow := &buttons[len(buttons)-1]
		*currentRow = append(*currentRow, models.InlineKeyboardButton{
			Text: getLocationButtonText(loc, selectedLocationCode), CallbackData: FormatPingCallbackData(loc.Id),
		})
	}

	return &models.InlineKeyboardMarkup{InlineKeyboard: buttons}
}

func handlePing(ctx context.Context, b *bot.Bot, update *models.Update) {
	provider := &MockPingEventsProvider{}
	statsWriter := &PingStatisticsBuilder{}
	streamInterval := ctx.Value(CtxKeyTxtStreamIntv).(time.Duration)

	if update.Message != nil {
		locationCode := ""
		allLocs := provider.GetAllLocations(ctx)
		if len(allLocs) > 0 {
			locationCode = allLocs[0].Id
		}

		txt := "Ping is starting..."
		// Send initial message with buttons
		msg, _ := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   txt,
			Entities: []models.MessageEntity{
				{
					Type:   models.MessageEntityTypePre,
					Offset: 0,
					Length: len(txt),
				},
			},
			ReplyMarkup: GetLocationButtons(ctx, locationCode, provider, ctx.Value(CtxKeyTGBtnLayoutCol).(int)),
		})

		// Emulate network latency and middleware overhead
		time.Sleep(1000 * time.Millisecond)

		for ev := range provider.GetEventsByLocationCode(ctx, locationCode) {
			statsWriter.WriteEvent(ev)
			txt := statsWriter.GetHumanReadableText()
			// Edit the original message with the statistics
			b.EditMessageText(ctx, &bot.EditMessageTextParams{
				ChatID:    update.Message.Chat.ID,
				MessageID: msg.ID,
				Text:      txt,
				Entities: []models.MessageEntity{
					{
						Type:   models.MessageEntityTypePre,
						Offset: 0,
						Length: len(txt),
					},
				},
				ReplyMarkup: GetLocationButtons(ctx, locationCode, provider, ctx.Value(CtxKeyTGBtnLayoutCol).(int)),
			})
			<-time.After(streamInterval)
		}
	}
}

func handlePingQueryCallback(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update == nil || update.CallbackQuery == nil {
		return
	}

	streamInterval := ctx.Value(CtxKeyTxtStreamIntv).(time.Duration)
	provider := &MockPingEventsProvider{}
	statsWriter := &PingStatisticsBuilder{}

	activeLocationCode := ParseLocationCodeFromPingCallbackData(update.CallbackQuery.Data)

	txt := "Ping is starting..."
	b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    update.CallbackQuery.Message.Message.Chat.ID,
		MessageID: update.CallbackQuery.Message.Message.ID,
		Text:      txt,
		Entities: []models.MessageEntity{
			{
				Type:   models.MessageEntityTypePre,
				Offset: 0,
				Length: len(txt),
			},
		},
		ReplyMarkup: GetLocationButtons(ctx, activeLocationCode, provider, ctx.Value(CtxKeyTGBtnLayoutCol).(int)),
	})

	// Emulate network latency and middleware overhead
	time.Sleep(1000 * time.Millisecond)

	for ev := range provider.GetEventsByLocationCode(ctx, activeLocationCode) {
		statsWriter.WriteEvent(ev)
		txt := statsWriter.GetHumanReadableText()
		// Edit the original message with the statistics
		b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    update.CallbackQuery.Message.Message.Chat.ID,
			MessageID: update.CallbackQuery.Message.Message.ID,
			Text:      txt,
			Entities: []models.MessageEntity{
				{
					Type:   models.MessageEntityTypePre,
					Offset: 0,
					Length: len(txt),
				},
			},
			ReplyMarkup: GetLocationButtons(ctx, activeLocationCode, provider, ctx.Value(CtxKeyTGBtnLayoutCol).(int)),
		})
		<-time.After(streamInterval)
	}

	// Answer the callback query to remove the loading state (only once, after all updates)
	b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
	})
}

func handleUptime(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message != nil {
		startedAt := ctx.Value(pkgutils.CtxKeyStartedAt).(time.Time)
		uptime := time.Since(startedAt)
		txt := fmt.Sprintf("Started at: %s\nUptime: %s", startedAt.Format(time.RFC3339), uptime.String())
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   txt,
			Entities: []models.MessageEntity{
				{
					Type:   models.MessageEntityTypePre,
					Offset: 0,
					Length: len(txt),
				},
			},
		})
	}
}

func getSubjectFromMessage(message *models.Message) string {
	if message.Chat.Type == models.ChatTypePrivate {
		return message.Chat.Username
	} else if message.Chat.Type == models.ChatTypeGroup || message.Chat.Type == models.ChatTypeSupergroup {
		return message.Chat.Title
	}
	return ""
}

func handleToken(ctx context.Context, b *bot.Bot, update *models.Update) {
	secret := ctx.Value(CtxKeyJWTSecret).([]byte)
	if secret == nil {
		panic("JWT secret is not set in the context")
	}

	issuerName := ctx.Value(CtxKeyIssuerName)
	if issuerName == nil {
		panic("Issuer name is not set in the context")
	}

	if update.Message != nil {
		if update.Message.Chat.Type == models.ChatTypePrivate {
			issuer := issuerName.(string)
			subject := getSubjectFromMessage(update.Message)
			if subject == "" {
				b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: update.Message.Chat.ID,
					Text:   "Error: Failed to get subject from message",
				})
				return
			}
			subject = fmt.Sprintf("telegram:@%s", subject)

			tokenId := uuid.New().String()

			tokenObject := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
				Issuer:   issuer,
				Subject:  subject,
				IssuedAt: jwt.NewNumericDate(time.Now()),
				ID:       tokenId,
			})

			tokenString, err := tokenObject.SignedString(secret)
			if err != nil {
				b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: update.Message.Chat.ID,
					Text:   fmt.Sprintf("Error: Failed to sign token: %v", err.Error()),
				})
				return
			}

			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   fmt.Sprintf("Token: %s", tokenString),
			})
			defer log.Printf("issued token for %s, token id: %s", subject, tokenId)
		}
	}
}
