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
	pkgtuitraceroute "github.com/internetworklab/cloudping/pkg/tui/traceroute"
)

type TracerouteCLI struct {
	IPv4        bool   `short:"4" name:"prefer-ipv4" help:"Use IPv4"`
	IPv6        bool   `short:"6" name:"prefer-ipv6" help:"Use IPv6"`
	Count       int    `short:"c" name:"count" help:"Number of packets to send" default:"24"`
	Destination string `arg:"" name:"destination" help:"Destination to trace"`
}

type TracerouteCommandHandler struct {
	LocationsProvider   pkgtui.LocationsProvider
	ConversationManager *pkgbot.ConversationManager
	PingEventsProvider  pkgtui.PingEventsProvider
	StreamIntv          time.Duration
	ButtonLayoutCols    int
}

func (handler *TracerouteCommandHandler) getBtnLayoutCols() int {
	if handler.ButtonLayoutCols <= 0 {
		return DefaultBtnLayoutCols
	}
	return handler.ButtonLayoutCols
}

func (handler *TracerouteCommandHandler) GetUsage() string {
	return "[-4] [-6] [-c <count>] <destination>"
}

func (handler *TracerouteCommandHandler) parseCLIString(cliString string) (*TracerouteCLI, error) {
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

	tracerouteCLI := &TracerouteCLI{}
	exitCh := make(chan int, 1)
	defer close(exitCh)

	kongInstance := kong.Must(
		tracerouteCLI,
		kong.Writers(helpBuff, helpBuff),
		kong.Name(""),
		kong.Exit(func(code int) {
			exitCh <- code
		}),
	)

	getHelp := func() string {
		select {
		case <-exitCh:
			return helpBuff.String()
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

	return tracerouteCLI, nil
}

func (handler *TracerouteCommandHandler) formatCallbackQuery(loc pkgtui.LocationDescriptor) string {
	return fmt.Sprintf("trace_location_%s", loc.Id)
}

func (handler *TracerouteCommandHandler) parseCallbackQuery(pingCallbackData string) string {
	if suffix, found := strings.CutPrefix(pingCallbackData, "trace_location_"); found {
		return suffix
	}
	return ""
}

// doHandleTraceroute runs the traceroute event loop. Returns true if exited
// normally (all events consumed), false if cancelled via context.
func (handler *TracerouteCommandHandler) doHandleTraceroute(ctx context.Context, tracerouteCLI *TracerouteCLI, src string, updateMessage func(msg string) error) {
	statsWriter := pkgtuitraceroute.NewTraceStatsBuilder()
	provider := handler.PingEventsProvider

	pingRequest := &pkgtui.PingRequestDescriptor{
		PreferV4:     tracerouteCLI.IPv4,
		PreferV6:     tracerouteCLI.IPv6,
		Sources:      []string{src},
		Destinations: []string{tracerouteCLI.Destination},
		Count:        tracerouteCLI.Count,
		Traceroute:   true,
	}
	evDataCh := provider.GetEvents(ctx, pingRequest)
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-evDataCh:
			if !ok {
				return
			}
			statsWriter.WriteEvent(ev)
			txt := statsWriter.GetHumanReadableText()
			err := updateMessage(txt)
			if err != nil {
				log.Printf("failed to edit message: %v", err)
			}
			<-time.After(handler.StreamIntv)
		}
	}
}

func (handler *TracerouteCommandHandler) HandleTraceroute(ctx context.Context, b *bot.Bot, update *models.Update) {
	provider := handler.PingEventsProvider
	conversationMng := handler.ConversationManager

	if update.Message == nil {
		return
	}

	replyParams := &models.ReplyParameters{
		ChatID:    update.Message.Chat.ID,
		MessageID: update.Message.ID,
	}

	cliString := update.Message.Text

	LogCommand(update, cliString)

	tracerouteCLI, err := handler.parseCLIString(cliString)
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

	destination := tracerouteCLI.Destination

	locationCode := ""
	allLocs, err := provider.GetAllLocations(ctx)
	if err != nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			Text:            fmt.Sprintf("Can't get locations: %s", err.Error()),
			ReplyParameters: replyParams,
		})
		return
	}
	if len(allLocs) > 0 {
		locationCode = allLocs[0].Id
	}

	buttons := GetLocationButtons(ctx, locationCode, provider, handler.getBtnLayoutCols(), handler.formatCallbackQuery)

	txt := fmt.Sprintf("Traceroute to %s is starting...", destination)
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
		DateTime:       time.Unix(int64(update.Message.Date), 0),
		Content:        update.Message.Text,
		InitialCommand: tracerouteCLI,
	}
	ctx, canceller := context.WithCancel(ctx)
	if err := conversationMng.CheckIn(ctx, conversationKey, initialMessage, canceller); err != nil {
		log.Printf("failed to checkin, conversationKey=%q", conversationKey.String())
	}

	<-time.After(handler.StreamIntv)

	updateMessage := func(txt string) error {
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
		return err
	}

	handler.doHandleTraceroute(ctx, tracerouteCLI, locationCode, updateMessage)
}

func (handler *TracerouteCommandHandler) HandleTraceQueryCallback(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update == nil || update.CallbackQuery == nil {
		return
	}

	provider := handler.PingEventsProvider
	convMngr := handler.ConversationManager

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

	if len(histMsgs) == 0 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatId,
			Text:   "Missing history context, please start a new conversation",
		})
		return
	}

	cliString := histMsgs[0].Content
	tracerouteCLI, err := handler.parseCLIString(cliString)
	if err != nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatId,
			Text:   fmt.Sprintf("Error: %v. Usage: %s", err, handler.GetUsage()),
		})
		return
	}

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

	if err := doEditMsg(chatId, msgId, fmt.Sprintf("Traceroute to %s is starting...", tracerouteCLI.Destination)); err != nil {
		log.Printf("failed to edit message: %v", err)
	}

	<-time.After(handler.StreamIntv)

	handler.doHandleTraceroute(ctx, tracerouteCLI, activeLocationCode, func(msg string) error { return doEditMsg(chatId, msgId, msg) })
}
