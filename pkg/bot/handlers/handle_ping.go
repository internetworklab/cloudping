package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	pkgbot "example.com/rbmq-demo/pkg/bot"
	pkgutils "example.com/rbmq-demo/pkg/utils"
	"github.com/alecthomas/kong"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type PingCLI struct {
	IPv4        bool   `short:"4" name:"prefer-ipv4" help:"Use IPv4"`
	IPv6        bool   `short:"6" name:"prefer-ipv6" help:"Use IPv6"`
	Count       int    `short:"c" name:"count" help:"Number of packets to send" default:"5"`
	Destination string `arg:"" name:"destination" help:"Destination to ping"`
}

type PingCommandHandler struct{}

func (handler *PingCommandHandler) GetUsage() string {
	return "/ping [-4] [-6] [-c <count>] <destination>"
}

func (handler *PingCommandHandler) parseCLIString(cliString string) (*PingCLI, *kong.Context, error) {
	cliSegs := pkgutils.SplitBySpace(cliString)
	if len(cliSegs) == 0 {
		return nil, nil, errors.New("no arguments provided")
	}

	pingCLI := new(PingCLI)
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

func (handler *PingCommandHandler) formatCallbackQuery(loc pkgbot.LocationDescriptor) string {
	return fmt.Sprintf("ping_location_%s", loc.Id)
}

func (handler *PingCommandHandler) parseCallbackQuery(pingCallbackData string) string {
	if suffix, found := strings.CutPrefix(pingCallbackData, "ping_location_"); found {
		return suffix
	}
	return ""
}

func (handler *PingCommandHandler) HandlePing(ctx context.Context, b *bot.Bot, update *models.Update) {
	provider := ctx.Value(CtxKeyPingEVProvider).(pkgbot.PingEventsProvider)
	statsWriter := &PingStatisticsBuilder{}
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

func (handler *PingCommandHandler) HandlePingQueryCallback(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update == nil || update.CallbackQuery == nil {
		return
	}

	streamInterval := ctx.Value(CtxKeyTxtStreamIntv).(time.Duration)
	provider := ctx.Value(CtxKeyPingEVProvider).(pkgbot.PingEventsProvider)
	convMngr := ctx.Value(CtxKeyConversationManager).(*pkgbot.ConversationManager)
	statsWriter := &PingStatisticsBuilder{}

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

	if err := doEditMsg(chatId, msgId, fmt.Sprintf("Ping to %s is starting...", destination)); err != nil {
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
			if err := doEditMsg(chatId, msgId, txt); err != nil {
				log.Printf("failed to edit message: %v", err)
			}
			<-time.After(streamInterval)
		}
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
	pingEvs          []pkgbot.PingEvent
	receivedPktCount int
	lossPktCount     int
	minRTT           int
	maxRTT           int
	totalRTT         int
}

func (statsBuilder *PingStatisticsBuilder) WriteEvent(ev pkgbot.PingEvent) {
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
