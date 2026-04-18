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

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	pkgbot "github.com/internetworklab/cloudping/pkg/bot"
	pkgbotdata "github.com/internetworklab/cloudping/pkg/bot/datasource"
	pkgbothandlers "github.com/internetworklab/cloudping/pkg/bot/handlers"
	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
)

// Please note, sensitive data such as token are provided via env, not presented in the command line.
type BotCmd struct {
	ListenAddress            string        `help:"Address to listen on." type:"string" default:":8083"`
	PublicEndpoint           string        `help:"Public endpoint of the bot." type:"string"`
	JWTAuthSecretFromEnv     string        `name:"jwt-auth-secret-from-env" help:"Name of the environment variable that contains the JWT secret, this secret is for issuing JWT tokens, not for authenticate itself with remote endpoint." default:"JWT_SECRET"`
	JWTAuthSecretFromFile    string        `name:"jwt-auth-secret-from-file" help:"Path to the file that contains the JWT secret, this secret is for issuing JWT tokens, not for authenticate itself with remote endpoint."`
	JWTIssuerName            string        `name:"jwt-issuer-name" help:"The issuer appeared in the signed jwt token" default:"globalping-hub"`
	TelegramWebhookSecretEnv string        `name:"tg-webhook-secret-env" help:"Name of the environment variable that stores the Telegram webhook secret" default:"TG_WS_SECRET"`
	TelegramBotSecretEnv     string        `name:"tg-bot-secret-env" help:"Name of the environment variable that stores the telegram bot secret" default:"TG_BOT_TOKEN"`
	TextStreamInterval       time.Duration `name:"tg-bot-text-stream-interval" help:"Sleeping interval between two consecutive Telegram bot text edit" default:"1500ms"`
	ButtonLayoutColumns      int           `name:"tg-bot-button-layout-columns" help:"Specify the number of columns of the layout of buttons grid of the bot's response message" default:"4"`
	PingResolver             string        `name:"ping-resolver" help:"Resolver being used to resolver hostname to IP address during an ICMP ping or traceroute task" default:"172.20.0.53:53"`
	UpstreamJWTSecretFromEnv string        `name:"upstream-jwt-sec-env" help:"Name of the enviornment variable that stores the JWT token use to authenticate with the upstream ping events provider" default:"UPSTREAM_JWT_TOKEN"`
	UpstreamAPIPrefix        string        `name:"upstream-api-prefix" help:"The API prefix of the upstream server where to get ping events data" default:"https://ping2.sh/api"`
	CustomFontNames          []string      `name:"custom-font-names" help:"Customize font names to search"`
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

func (botCmd *BotCmd) Run(sharedCtx *pkgutils.GlobalSharedContext) error {
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

	pingEVProvider := &pkgbotdata.CloudPingEventsProvider{
		APIPrefix: botCmd.UpstreamAPIPrefix,
		JWTToken:  os.Getenv(botCmd.UpstreamJWTSecretFromEnv),
		Resolver:  botCmd.PingResolver,
	}

	convMngr := &pkgbot.ConversationManager{}

	startedAt := time.Now()
	ctx = context.WithValue(ctx, pkgutils.CtxKeyStartedAt, startedAt)
	ctx = context.WithValue(ctx, pkgbothandlers.CtxKeyBuildVersion, sharedCtx.BuildVersion)
	ctx = context.WithValue(ctx, pkgbothandlers.CtxKeyJWTSecret, secret)
	ctx = context.WithValue(ctx, pkgbothandlers.CtxKeyTxtStreamIntv, botCmd.TextStreamInterval)
	ctx = context.WithValue(ctx, pkgbothandlers.CtxKeyIssuerName, botCmd.JWTIssuerName)
	ctx = context.WithValue(ctx, pkgbothandlers.CtxKeyTGBtnLayoutCol, botCmd.ButtonLayoutColumns)
	ctx = context.WithValue(ctx, pkgbothandlers.CtxKeyPingEVProvider, pingEVProvider)
	ctx = context.WithValue(ctx, pkgbothandlers.CtxKeyConversationManager, convMngr)

	botPingCmdHandler := &pkgbothandlers.PingCommandHandler{}
	traceCmdHandler := &pkgbothandlers.TracerouteCommandHandler{}
	probeHandler := &pkgbothandlers.ProbeHandler{
		FontNames:           botCmd.CustomFontNames,
		LocationsProvider:   pingEVProvider,
		ProbeEventsProvider: pingEVProvider,
		ConversationManager: convMngr,
	}
	listHandler := &pkgbothandlers.ListHandler{
		EventsProvider: pingEVProvider,
	}

	b.RegisterHandlerRegexp(bot.HandlerTypeMessageText, regexp.MustCompile(`^/start`), pkgbothandlers.HandleStart)
	b.RegisterHandlerRegexp(bot.HandlerTypeMessageText, regexp.MustCompile(`^/ping`), botPingCmdHandler.HandlePing)
	b.RegisterHandlerRegexp(bot.HandlerTypeMessageText, regexp.MustCompile(`^/traceroute`), traceCmdHandler.HandleTraceroute)
	b.RegisterHandlerRegexp(bot.HandlerTypeMessageText, regexp.MustCompile(`^/uptime`), pkgbothandlers.HandleUptime)
	b.RegisterHandlerRegexp(bot.HandlerTypeMessageText, regexp.MustCompile(`^/token`), pkgbothandlers.HandleToken)
	b.RegisterHandlerRegexp(bot.HandlerTypeMessageText, regexp.MustCompile(`^/version`), pkgbothandlers.HandleVersion)
	b.RegisterHandlerRegexp(bot.HandlerTypeMessageText, regexp.MustCompile(`^/probe`), probeHandler.HandleProbe)
	b.RegisterHandlerRegexp(bot.HandlerTypeMessageText, regexp.MustCompile(`^/list`), listHandler.HandleList)
	b.RegisterHandlerRegexp(bot.HandlerTypeCallbackQueryData, regexp.MustCompile(`^ping_location_.+$`), botPingCmdHandler.HandlePingQueryCallback)
	b.RegisterHandlerRegexp(bot.HandlerTypeCallbackQueryData, regexp.MustCompile(`^trace_location_.+$`), traceCmdHandler.HandleTraceQueryCallback)
	b.RegisterHandlerRegexp(bot.HandlerTypeCallbackQueryData, regexp.MustCompile(`^probe_cancel$`), probeHandler.HandleProbeCancelQueryCallback)

	_, err = b.SetMyCommands(ctx, &bot.SetMyCommandsParams{
		Commands: []models.BotCommand{
			{Command: "/start", Description: "No op, just a placeholder."},
			{Command: "/ping", Description: "Usage: " + botPingCmdHandler.GetUsage()},
			{Command: "/traceroute", Description: "Usage: " + traceCmdHandler.GetUsage()},
			{Command: "/version", Description: "Show build version information."},
			{Command: "/probe", Description: "Probe specified CIDR. Usage: " + probeHandler.GetUsage()},
			{Command: "/list", Description: "List all available nodes"},
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
