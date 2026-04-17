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
	pkgtable "github.com/internetworklab/cloudping/pkg/table"
	pkgtuitraceroute "github.com/internetworklab/cloudping/pkg/tui/traceroute"
	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
)

type TracerouteCLI struct {
	IPv4        bool   `short:"4" name:"prefer-ipv4" help:"Use IPv4"`
	IPv6        bool   `short:"6" name:"prefer-ipv6" help:"Use IPv6"`
	Count       int    `short:"c" name:"count" help:"Number of packets to send" default:"24"`
	Destination string `arg:"" name:"destination" help:"Destination to trace"`
}

type TracerouteCommandHandler struct{}

func (handler *TracerouteCommandHandler) GetUsage() string {
	return "/traceroute [-4] [-6] [-c <count>] <destination>"
}

func (handler *TracerouteCommandHandler) parseCLIString(cliString string) (*TracerouteCLI, *kong.Context, error) {
	cliSegs := pkgutils.SplitBySpace(cliString)
	if len(cliSegs) == 0 {
		return nil, nil, errors.New("no arguments provided")
	}

	pingCLI := new(TracerouteCLI)
	parser, err := kong.New(pingCLI)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create kong parser: %w", err)
	}

	kongCtx, err := parser.Parse(cliSegs)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse ping CLI: %w", err)
	}

	return pingCLI, kongCtx, nil
}

func (handler *TracerouteCommandHandler) formatCallbackQuery(loc pkgbot.LocationDescriptor) string {
	return fmt.Sprintf("trace_location_%s", loc.Id)
}

func (handler *TracerouteCommandHandler) parseCallbackQuery(pingCallbackData string) string {
	if suffix, found := strings.CutPrefix(pingCallbackData, "trace_location_"); found {
		return suffix
	}
	return ""
}

func (handler *TracerouteCommandHandler) HandleTraceroute(ctx context.Context, b *bot.Bot, update *models.Update) {
	provider := ctx.Value(CtxKeyPingEVProvider).(pkgbot.PingEventsProvider)
	statsWriter := pkgtuitraceroute.NewTraceStatsBuilder()
	streamInterval := ctx.Value(CtxKeyTxtStreamIntv).(time.Duration)
	conversationMng := ctx.Value(CtxKeyConversationManager).(*pkgbot.ConversationManager)

	if update.Message != nil {
		cliString := pkgbot.TrimCommandPrefix(update.Message.Text)
		pingCLI, _, err := handler.parseCLIString(cliString)
		if err != nil {
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   fmt.Sprintf("Error: %v. Usage: %s", err, handler.GetUsage()),
			})
			return
		}
		destination := pingCLI.Destination

		locationCode := ""
		allLocs, err := provider.GetAllLocations(ctx)
		if err != nil {
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   fmt.Sprintf("Can't get locations: %s", err.Error()),
			})
			return
		}
		if len(allLocs) > 0 {
			locationCode = allLocs[0].Id
		}
		buttons := GetLocationButtons(ctx, locationCode, provider, ctx.Value(CtxKeyTGBtnLayoutCol).(int), handler.formatCallbackQuery)

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
			ReplyMarkup: buttons,
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

		pingRequest := &pkgbot.PingRequestDescriptor{
			PreferV4:     pingCLI.IPv4,
			PreferV6:     pingCLI.IPv6,
			Sources:      []string{locationCode},
			Destinations: []string{destination},
			Count:        pingCLI.Count,
			Traceroute:   true,
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
}

func (handler *TracerouteCommandHandler) HandleTraceQueryCallback(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update == nil || update.CallbackQuery == nil {
		return
	}

	streamInterval := ctx.Value(CtxKeyTxtStreamIntv).(time.Duration)
	provider := ctx.Value(CtxKeyPingEVProvider).(pkgbot.PingEventsProvider)
	convMngr := ctx.Value(CtxKeyConversationManager).(*pkgbot.ConversationManager)
	statsWriter := pkgtuitraceroute.NewTraceStatsBuilder()

	activeLocationCode := handler.parseCallbackQuery(update.CallbackQuery.Data)

	chatId := update.CallbackQuery.Message.Message.Chat.ID
	msgId := update.CallbackQuery.Message.Message.ID
	conversationKey := pkgbot.ConversationKey{
		ChatId: chatId,
		MsgId:  msgId,
		FromId: update.CallbackQuery.From.ID,
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

	cliString := pkgbot.TrimCommandPrefix(histMsgs[0].Content)
	pingCLI, _, err := handler.parseCLIString(cliString)
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
			ReplyMarkup: GetLocationButtons(ctx, activeLocationCode, provider, ctx.Value(CtxKeyTGBtnLayoutCol).(int), handler.formatCallbackQuery),
		})
		return err
	}

	if err := doEditMsg(chatId, msgId, fmt.Sprintf("Traceroute to %s is starting...", destination)); err != nil {
		log.Printf("failed to edit message: %v", err)
	}

	// Emulate network latency and middleware overhead
	time.Sleep(1000 * time.Millisecond)

	pingRequest := &pkgbot.PingRequestDescriptor{
		PreferV4:     pingCLI.IPv4,
		PreferV6:     pingCLI.IPv6,
		Sources:      []string{activeLocationCode},
		Destinations: []string{destination},
		Count:        pingCLI.Count,
		Traceroute:   true,
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

func (handler *TracerouteCommandHandler) getExampleTable() pkgtable.Table {
	// Write header rows
	table := pkgtable.Table{}
	table.Rows = append(
		table.Rows,
		pkgtable.Row{Cells: []string{"Hop", "Peer", "RTTs (Last Min/Avg/Max)", "Stats (Rx/Tx/Loss)"}},
		pkgtable.Row{Cells: []string{"", "(IP address)", "ASN Network", "City,Country"}},
		pkgtable.Row{Cells: []string{"", "", "", ""}},
		pkgtable.Row{Cells: []string{"1.", "homelab.local", "1ms 1ms/2ms/3ms", "2/3/33%"}},
		pkgtable.Row{Cells: []string{"", "(192.168.1.1)", "", ""}},
		pkgtable.Row{Cells: []string{"", "", "", ""}},
		pkgtable.Row{Cells: []string{"2.", "a.example.com", "10ms 10ms/10ms/10ms", "3/3/0%"}},
		pkgtable.Row{Cells: []string{"", "(17.18.19.20)", "AS12345 Example LLC", "HongKong,HK"}},
		pkgtable.Row{Cells: []string{}},
		pkgtable.Row{Cells: []string{"3.", "[TIMEOUT]", "", ""}},
		pkgtable.Row{Cells: []string{"", "(*)", "", ""}},
		pkgtable.Row{Cells: []string{}},
		pkgtable.Row{Cells: []string{"4.", "google.com", "100ms 100ms/100ms/100ms", "1/1/0%"}},
	)

	return table
}
