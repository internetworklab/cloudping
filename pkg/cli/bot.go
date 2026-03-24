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

	pkgbot "example.com/rbmq-demo/pkg/bot"
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
	CtxKeyJWTSecret           = CtxKey("jwt_secret")
	CtxKeyIssuerName          = CtxKey("issuer_name")
	CtxKeyTxtStreamIntv       = CtxKey("txt_stream_intv")
	CtxKeyTGBtnLayoutCol      = CtxKey("tg_btn_layout_col")
	CtxKeyPingEVProvider      = CtxKey("ping_ev_provider")
	CtxKeyConversationManager = CtxKey("conv_mng")
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

	var pingEVProvider pkgbot.PingEventsProvider
	pingEVProvider = &pkgbot.MockPingEventsProvider{}

	convMngr := &pkgbot.ConversationManager{}

	startedAt := time.Now()
	ctx = context.WithValue(ctx, pkgutils.CtxKeyStartedAt, startedAt)
	ctx = context.WithValue(ctx, CtxKeyTxtStreamIntv, botCmd.TextStreamInterval)
	ctx = context.WithValue(ctx, CtxKeyIssuerName, botCmd.JWTIssuerName)
	ctx = context.WithValue(ctx, CtxKeyTGBtnLayoutCol, botCmd.ButtonLayoutColumns)
	ctx = context.WithValue(ctx, CtxKeyPingEVProvider, pingEVProvider)
	ctx = context.WithValue(ctx, CtxKeyConversationManager, convMngr)

	b.RegisterHandlerRegexp(bot.HandlerTypeMessageText, regexp.MustCompile(`^/start`), handleStart)
	b.RegisterHandlerRegexp(bot.HandlerTypeMessageText, regexp.MustCompile(`^/ping`), handlePing)
	b.RegisterHandlerRegexp(bot.HandlerTypeMessageText, regexp.MustCompile(`^/uptime`), handleUptime)
	b.RegisterHandlerRegexp(bot.HandlerTypeMessageText, regexp.MustCompile(`^/token`), handleToken)
	b.RegisterHandlerRegexp(bot.HandlerTypeCallbackQueryData, regexp.MustCompile(`^ping_location_.+$`), handlePingQueryCallback)

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
			_, err := b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   "Already started!",
			})
			if err != nil {
				log.Printf("failed to send message: %v", err)
			}
		}
	}
}

// getLocationButtonText returns the button text for a ping task, with a checkmark if selected.
func getLocationButtonText(loc pkgbot.LocationDescriptor, activeLocationCode string) string {
	activationMark := ""
	if loc.Id == activeLocationCode {
		activationMark = " ✓"
	}
	return pkgutils.Alpha2CountryCodeToUnicode(loc.Alpha2CountryCode) + loc.Label + activationMark
}

// GetLocationButtons returns an inline keyboard markup with location buttons,
// showing a checkmark indicator on the currently selected location.
func GetLocationButtons(ctx context.Context, selectedLocationCode string, provider pkgbot.PingEventsProvider, numCols int) *models.InlineKeyboardMarkup {
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
			Text: getLocationButtonText(loc, selectedLocationCode), CallbackData: pkgbot.FormatPingCallbackData(loc.Id),
		})
	}

	return &models.InlineKeyboardMarkup{InlineKeyboard: buttons}
}

func extractDestinationFromMessage(updateText string) string {

	// Check if the message starts with /ping and extract the destination
	text := strings.TrimSpace(updateText)
	rest, found := strings.CutPrefix(text, "/ping ")
	if !found {
		return ""
	}

	// Trim any leading/trailing whitespace from the destination
	destination := strings.TrimSpace(rest)
	return destination
}

func handlePing(ctx context.Context, b *bot.Bot, update *models.Update) {
	provider := ctx.Value(CtxKeyPingEVProvider).(pkgbot.PingEventsProvider)
	statsWriter := &pkgbot.PingStatisticsBuilder{}
	streamInterval := ctx.Value(CtxKeyTxtStreamIntv).(time.Duration)
	conversationMng := ctx.Value(CtxKeyConversationManager).(*pkgbot.ConversationManager)

	if update.Message != nil {
		destination := extractDestinationFromMessage(update.Message.Text)
		if destination == "" {
			_, err := b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   "Error: Destination cannot be omitted. Usage: /ping <destination>",
			})
			if err != nil {
				log.Printf("failed to send message: %v", err)
			}
			return
		}

		locationCode := ""
		allLocs := provider.GetAllLocations(ctx)
		if len(allLocs) > 0 {
			locationCode = allLocs[0].Id
		}

		txt := fmt.Sprintf("Ping to %s is starting...", destination)
		// Send initial message with buttons
		msg, err := b.SendMessage(ctx, &bot.SendMessageParams{
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
		if err != nil {
			log.Printf("failed to send message: %v", err)
		}
		conversationKey := &pkgbot.ConversationKey{
			ChatId: update.Message.Chat.ID,
			FromId: update.Message.From.ID,
			MsgId:  msg.ID,
		}

		initialMessage := pkgbot.MessageRecord{
			DateTime: time.Unix(int64(update.Message.Date), 0),
			Content:  update.Message.Text,
		}
		ctx, canceller := context.WithCancel(ctx)
		if err := conversationMng.CheckIn(ctx, conversationKey, initialMessage, canceller); err != nil {
			log.Printf("failed to checkin, conversationKey=%q", conversationKey.String())
		}

		// Emulate network latency and middleware overhead
		time.Sleep(1000 * time.Millisecond)

		evDataCh := provider.GetEventsByLocationCodeAndDestination(ctx, locationCode, destination)
		for {
			select {
			case <-ctx.Done():
				log.Printf("conversationKey=%q, cancelled", conversationKey.String())
				return
			case ev, ok := <-evDataCh:
				if !ok {
					// no more data to consume
					return
				}
				statsWriter.WriteEvent(ev)
				txt := statsWriter.GetHumanReadableText()
				// Edit the original message with the statistics
				_, err = b.EditMessageText(ctx, &bot.EditMessageTextParams{
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
				if err != nil {
					log.Printf("failed to edit message: %v", err)
				}
				<-time.After(streamInterval)
			}
		}
	}
}

func handlePingQueryCallback(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update == nil || update.CallbackQuery == nil {
		return
	}

	streamInterval := ctx.Value(CtxKeyTxtStreamIntv).(time.Duration)
	provider := ctx.Value(CtxKeyPingEVProvider).(pkgbot.PingEventsProvider)
	convMngr := ctx.Value(CtxKeyConversationManager).(*pkgbot.ConversationManager)
	statsWriter := &pkgbot.PingStatisticsBuilder{}

	activeLocationCode := pkgbot.ParseLocationCodeFromPingCallbackData(update.CallbackQuery.Data)

	conversationKey := pkgbot.ConversationKey{
		ChatId: update.CallbackQuery.Message.Message.Chat.ID,
		FromId: update.CallbackQuery.From.ID,
		MsgId:  update.CallbackQuery.Message.Message.ID,
	}
	ctx, canceller := context.WithCancel(ctx)
	histMsgs, err := convMngr.CutIn(ctx, &conversationKey, canceller)
	if err != nil {
		log.Printf("failed to cut in, conversationKey=%q", conversationKey.String())
	}

	var destination string = ""
	if len(histMsgs) > 0 {
		destination = extractDestinationFromMessage(histMsgs[0].Content)
	}

	txt := fmt.Sprintf("Ping to %s is starting...", destination)
	_, err = b.EditMessageText(ctx, &bot.EditMessageTextParams{
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
	if err != nil {
		log.Printf("failed to edit message: %v", err)
	}

	// Emulate network latency and middleware overhead
	time.Sleep(1000 * time.Millisecond)

	evDataCh := provider.GetEventsByLocationCodeAndDestination(ctx, activeLocationCode, destination)
	for {
		select {
		case <-ctx.Done():
			log.Printf("conversationKey=%q, cancelled", conversationKey.String())
			return
		case ev, ok := <-evDataCh:
			if !ok {
				// Answer the callback query to remove the loading state (only once, after all updates)
				_, err = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
					CallbackQueryID: update.CallbackQuery.ID,
				})
				if err != nil {
					log.Printf("failed to answer callback query: %v", err)
				}
				return
			}
			statsWriter.WriteEvent(ev)
			txt := statsWriter.GetHumanReadableText()
			// Edit the original message with the statistics
			_, err = b.EditMessageText(ctx, &bot.EditMessageTextParams{
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
			if err != nil {
				log.Printf("failed to edit message: %v", err)
			}
			<-time.After(streamInterval)
		}
	}
}

func handleUptime(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message != nil {
		startedAt := ctx.Value(pkgutils.CtxKeyStartedAt).(time.Time)
		uptime := time.Since(startedAt)
		txt := fmt.Sprintf("Started at: %s\nUptime: %s", startedAt.Format(time.RFC3339), uptime.String())
		_, err := b.SendMessage(ctx, &bot.SendMessageParams{
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
		if err != nil {
			log.Printf("failed to send message: %v", err)
		}
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
				_, err := b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: update.Message.Chat.ID,
					Text:   "Error: Failed to get subject from message",
				})
				if err != nil {
					log.Printf("failed to send message: %v", err)
				}
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
				_, sendErr := b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: update.Message.Chat.ID,
					Text:   fmt.Sprintf("Error: Failed to sign token: %v", err.Error()),
				})
				if sendErr != nil {
					log.Printf("failed to send message: %v", sendErr)
				}
				return
			}

			_, err = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   fmt.Sprintf("Token: %s", tokenString),
			})
			if err != nil {
				log.Printf("failed to send message: %v", err)
			}
			defer log.Printf("issued token for %s, token id: %s", subject, tokenId)
		}
	}
}
