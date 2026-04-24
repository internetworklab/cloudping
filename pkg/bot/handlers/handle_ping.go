package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	pkgbot "github.com/internetworklab/cloudping/pkg/bot"
	pkgtui "github.com/internetworklab/cloudping/pkg/tui"
	pkgtuiping "github.com/internetworklab/cloudping/pkg/tui/ping"
)

type PingCLI struct {
	From        []string `short:"s" name:"from" help:"To explicitly specify the location of probe(s)"`
	IPv4        bool     `short:"4" name:"prefer-ipv4" help:"Use IPv4"`
	IPv6        bool     `short:"6" name:"prefer-ipv6" help:"Use IPv6"`
	Count       int      `short:"c" name:"count" help:"Number of packets to send" default:"5"`
	Destination string   `arg:"" name:"destination" help:"Destination to ping"`
}

type PingCommandHandler struct {
	LocationsProvider   pkgtui.LocationsProvider
	ConversationManager *pkgbot.ConversationManager
	PingEventsProvider  pkgtui.PingEventsProvider
	StreamIntv          time.Duration
	ButtonLayoutCols    int
}

func (handler *PingCommandHandler) getBtnLayoutCols() int {
	if handler.ButtonLayoutCols <= 0 {
		return DefaultBtnLayoutCols
	}
	return handler.ButtonLayoutCols
}

func (handler *PingCommandHandler) GetUsage() string {
	return "/ping [-4] [-6] [--from|-s] [-c|--count] <destination>"
}

func (handler *PingCommandHandler) parseCLIString(cliString string) (*PingCLI, error) {
	// Buffer for storing help text
	helpBuff := &strings.Builder{}

	cliSegs := strings.Fields(cliString)
	if len(cliSegs) > 0 && strings.HasPrefix(cliSegs[0], "/") {
		// strip the first /-leading segment
		cliSegs = cliSegs[1:]
	}

	if len(cliSegs) == 0 {
		return nil, errors.New("no arguments provided")
	}

	pingCLI := &PingCLI{}
	exitCh := make(chan int, 1)
	defer close(exitCh)

	kongInstance := kong.Must(
		pingCLI,
		kong.Writers(helpBuff, helpBuff),
		kong.Name(""),
		kong.Exit(func(code int) {
			exitCh <- code
		}),
	)

	getHelp := func() string {
		select {
		case <-exitCh:
			return fmt.Sprintf("Help:\n%s", helpBuff.String())
		default:
			return ""
		}
	}

	_, err := kongInstance.Parse(cliSegs)
	if err != nil {
		fmt.Fprintf(helpBuff, "Error: %v", err)
		if help := getHelp(); help != "" {
			return nil, fmt.Errorf("Help:\n%s\n", help)
		} else {
			return nil, fmt.Errorf("Unknown error: %v", err)
		}
	}

	if help := getHelp(); help != "" {
		return nil, fmt.Errorf("Help:\n%s\n", help)
	}

	return pingCLI, nil
}

func (handler *PingCommandHandler) formatCallbackQuery(loc pkgtui.LocationDescriptor) string {
	return fmt.Sprintf("ping_location_%s", loc.Id)
}

func (handler *PingCommandHandler) parseCallbackQuery(pingCallbackData string) string {
	if suffix, found := strings.CutPrefix(pingCallbackData, "ping_location_"); found {
		return suffix
	}
	return ""
}

func (handler *PingCommandHandler) getSrcLoc(ctx context.Context, pingCLI *PingCLI) (string, error) {
	if len(pingCLI.From) > 0 {
		for _, s := range pingCLI.From {
			if s != "" {
				return s, nil
			}
		}
	}
	locationCode := ""
	allLocs, err := handler.LocationsProvider.GetAllLocations(ctx)
	if err != nil {
		return "", err

	}
	if len(allLocs) > 0 {
		locationCode = allLocs[0].Id
	}
	return locationCode, nil
}

func (handler *PingCommandHandler) doHandlePing(ctx context.Context, b *bot.Bot, update *models.Update, pingCLI *PingCLI) {
	statsWriter := &pkgtuiping.PingStatisticsBuilder{}
	streamInterval := handler.StreamIntv
	provider := handler.PingEventsProvider
	conversationMng := handler.ConversationManager

	destination := pingCLI.Destination

	replyParams := &models.ReplyParameters{
		ChatID:    update.Message.Chat.ID,
		MessageID: update.Message.ID,
	}

	locationCode, err := handler.getSrcLoc(ctx, pingCLI)
	if err != nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			Text:            fmt.Sprintf("Can't get locations: %s", err.Error()),
			ReplyParameters: replyParams,
		})
		return
	} else if locationCode == "" {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			Text:            fmt.Sprintf("Can't get locations"),
			ReplyParameters: replyParams,
		})
		return
	}

	buttons := GetLocationButtons(ctx, locationCode, provider, handler.getBtnLayoutCols(), handler.formatCallbackQuery)

	// Send initial message with buttons
	txt := fmt.Sprintf("Ping to %s is starting...", destination)
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
		ReplyMarkup:     buttons,
		ReplyParameters: replyParams,
	})
	if err != nil {
		log.Printf("failed to send message: %v", err)
	}
	conversationKey := &pkgbot.ConversationKey{
		ChatId: update.Message.Chat.ID,
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

	time.Sleep(streamInterval)

	pingRequest := &pkgtui.PingRequestDescriptor{
		PreferV4:     pingCLI.IPv4,
		PreferV6:     pingCLI.IPv6,
		Sources:      []string{locationCode},
		Destinations: []string{destination},
		Count:        pingCLI.Count,
	}
	evDataCh := provider.GetEvents(ctx, pingRequest)
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
				ReplyMarkup: buttons,
			})
			if err != nil {
				log.Printf("failed to edit message: %v", err)
			}
			<-time.After(streamInterval)
		}
	}
}

func (handler *PingCommandHandler) HandlePing(ctx context.Context, b *bot.Bot, update *models.Update) {

	if update.Message == nil {
		return
	}

	replyParams := &models.ReplyParameters{
		ChatID:    update.Message.Chat.ID,
		MessageID: update.Message.ID,
	}

	cliString := update.Message.Text
	pingCLI, err := handler.parseCLIString(cliString)
	if err != nil {
		helpText := err.Error()
		_, err = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			Text:            helpText,
			ReplyParameters: replyParams,
			Entities: []models.MessageEntity{
				{
					Type:   models.MessageEntityTypePre,
					Offset: 0,
					Length: len(helpText),
				},
			},
		})
		if err != nil {
			log.Printf("Failed to send message: %v", err)
		}
		return
	}

	handler.doHandlePing(ctx, b, update, pingCLI)
}

func (handler *PingCommandHandler) HandlePingQueryCallback(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update == nil || update.CallbackQuery == nil {
		return
	}

	streamInterval := handler.StreamIntv
	provider := handler.PingEventsProvider
	convMngr := handler.ConversationManager
	statsWriter := &pkgtuiping.PingStatisticsBuilder{}

	activeLocationCode := handler.parseCallbackQuery(update.CallbackQuery.Data)

	chatId := update.CallbackQuery.Message.Message.Chat.ID
	msgId := update.CallbackQuery.Message.Message.ID
	conversationKey := pkgbot.ConversationKey{
		ChatId: chatId,
		MsgId:  msgId,
	}
	ctx, canceller := context.WithCancel(ctx)
	defer canceller()
	histMsgs, err := convMngr.CutIn(ctx, &conversationKey, canceller)
	if err != nil {
		log.Printf("failed to cut in, conversationKey=%q", conversationKey.String())
		return
	}

	var destination string = ""
	if len(histMsgs) == 0 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatId,
			Text:   "Missing history context, please start a new conversation",
		})
		return
	}

	cliString := histMsgs[0].Content
	pingCLI, err := handler.parseCLIString(cliString)
	if err != nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatId,
			Text:   fmt.Sprintf("Error: %v. Usage: %s", err, handler.GetUsage()),
		})
		return
	}
	destination = pingCLI.Destination

	doEditMsg := func(chatId int64, msgId int, txt string) error {
		_, err = b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatId,
			MessageID: msgId,
			Text:      txt,
			Entities: []models.MessageEntity{
				{
					Type:   models.MessageEntityTypePre,
					Offset: 0,
					Length: len(txt),
				},
			},
			ReplyMarkup: GetLocationButtons(ctx, activeLocationCode, provider, handler.getBtnLayoutCols(), handler.formatCallbackQuery),
		})
		return err
	}

	if err := doEditMsg(chatId, msgId, fmt.Sprintf("Ping to %s is starting...", destination)); err != nil {
		log.Printf("failed to edit message: %v", err)
	}

	// Emulate network latency and middleware overhead
	time.Sleep(1000 * time.Millisecond)
	pingRequest := &pkgtui.PingRequestDescriptor{
		PreferV4:     pingCLI.IPv4,
		PreferV6:     pingCLI.IPv6,
		Sources:      []string{activeLocationCode},
		Destinations: []string{destination},
		Count:        pingCLI.Count,
	}
	evDataCh := provider.GetEvents(ctx, pingRequest)
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
			if err := doEditMsg(chatId, msgId, txt); err != nil {
				log.Printf("failed to edit message: %v", err)
			}
			<-time.After(streamInterval)
		}
	}
}
