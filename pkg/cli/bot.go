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
	"syscall"
	"time"

	pkgbot "example.com/rbmq-demo/pkg/bot"
	pkgbothandlers "example.com/rbmq-demo/pkg/bot/handlers"
	pkgutils "example.com/rbmq-demo/pkg/utils"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
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
		bot.WithDefaultHandler(pkgbothandlers.HandleDefault),
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

	var pingEVProvider pkgbot.PingEventsProvider = &pkgbot.MockPingEventsProvider{}

	convMngr := &pkgbot.ConversationManager{}

	startedAt := time.Now()
	ctx = context.WithValue(ctx, pkgutils.CtxKeyStartedAt, startedAt)
	ctx = context.WithValue(ctx, pkgbothandlers.CtxKeyJWTSecret, secret)
	ctx = context.WithValue(ctx, pkgbothandlers.CtxKeyTxtStreamIntv, botCmd.TextStreamInterval)
	ctx = context.WithValue(ctx, pkgbothandlers.CtxKeyIssuerName, botCmd.JWTIssuerName)
	ctx = context.WithValue(ctx, pkgbothandlers.CtxKeyTGBtnLayoutCol, botCmd.ButtonLayoutColumns)
	ctx = context.WithValue(ctx, pkgbothandlers.CtxKeyPingEVProvider, pingEVProvider)
	ctx = context.WithValue(ctx, pkgbothandlers.CtxKeyConversationManager, convMngr)

	b.RegisterHandlerRegexp(bot.HandlerTypeMessageText, regexp.MustCompile(`^/start`), pkgbothandlers.HandleStart)
	b.RegisterHandlerRegexp(bot.HandlerTypeMessageText, regexp.MustCompile(`^/ping`), pkgbothandlers.HandlePing)
	b.RegisterHandlerRegexp(bot.HandlerTypeMessageText, regexp.MustCompile(`^/uptime`), pkgbothandlers.HandleUptime)
	b.RegisterHandlerRegexp(bot.HandlerTypeMessageText, regexp.MustCompile(`^/token`), pkgbothandlers.HandleToken)
	b.RegisterHandlerRegexp(bot.HandlerTypeCallbackQueryData, regexp.MustCompile(`^ping_location_.+$`), pkgbothandlers.HandlePingQueryCallback)

	_, err = b.SetMyCommands(ctx, &bot.SetMyCommandsParams{
		Commands: []models.BotCommand{
			{Command: "/start", Description: "No op, just a placeholder."},
			{Command: "/ping", Description: "Usage: /ping <destination>"},
		},
	})
	if err != nil {
		log.Fatal("Failed to call setMyCommands", err)
		return err
	}

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
