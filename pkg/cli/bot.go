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

	pkgutils "example.com/rbmq-demo/pkg/utils"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Please note, sensitive data such as token are provided via env, not presented in the command line.
type BotCmd struct {
	ListenAddress         string `help:"Address to listen on." type:"string" default:":8083"`
	PublicEndpoint        string `help:"Public endpoint of the bot." type:"string"`
	JWTAuthSecretFromEnv  string `name:"jwt-auth-secret-from-env" help:"Name of the environment variable that contains the JWT secret" default:"JWT_SECRET"`
	JWTAuthSecretFromFile string `name:"jwt-auth-secret-from-file" help:"Path to the file that contains the JWT secret"`
}

func (botCmd *BotCmd) getJWTSecret() ([]byte, error) {
	return getJWTSecFromSomewhere(botCmd.JWTAuthSecretFromEnv, botCmd.JWTAuthSecretFromFile)
}

type CtxKey string

const (
	CtxKeyJWTSecret = CtxKey("jwt_secret")
)

func (botCmd *BotCmd) Run() error {

	secret, err := botCmd.getJWTSecret()
	if err != nil {
		return fmt.Errorf("failed to get JWT secret: %v", err)
	}

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ctx = context.WithValue(ctx, CtxKeyJWTSecret, secret)

	botToken := os.Getenv("TG_BOT_TOKEN")
	if botToken == "" {
		return fmt.Errorf("TG_BOT_TOKEN is not set")
	}

	tgWebSocketSecret := os.Getenv("TG_WS_SECRET")
	if tgWebSocketSecret == "" {
		return fmt.Errorf("TG_WS_SECRET is not set")
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

	b.RegisterHandlerRegexp(bot.HandlerTypeMessageText, regexp.MustCompile(`^/start`), handleStart)
	b.RegisterHandlerRegexp(bot.HandlerTypeMessageText, regexp.MustCompile(`^/ping`), handlePing)
	b.RegisterHandlerRegexp(bot.HandlerTypeMessageText, regexp.MustCompile(`^/uptime`), handleUptime)
	b.RegisterHandlerRegexp(bot.HandlerTypeMessageText, regexp.MustCompile(`^/token`), handleToken)

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

func handlePing(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message != nil {
		txt := "Pong!"
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
		panic("JWT secret is not set")
	}

	if update.Message != nil {
		if update.Message.Chat.Type == models.ChatTypePrivate {
			issuer := "globalping-hub"
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
