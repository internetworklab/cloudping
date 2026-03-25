package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	pkgbot "example.com/rbmq-demo/pkg/bot"
	pkgutils "example.com/rbmq-demo/pkg/utils"
	"github.com/alecthomas/kong"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type TracerouteCLI struct {
	IPv4        bool   `short:"4" name:"prefer-ipv4" help:"Use IPv4"`
	IPv6        bool   `short:"6" name:"prefer-ipv6" help:"Use IPv6"`
	Count       int    `short:"c" name:"count" help:"Number of packets to send" default:"5"`
	Destination string `arg:"" name:"destination" help:"Destination to trace"`
}

type TracerouteCommandHandler struct{}

func (handler *TracerouteCommandHandler) GetUsage() string {
	return "/traceroute [-4] [-6] [-c <count>] <destination>"
}

func (handler *TracerouteCommandHandler) parseCLIString(cliString string) (*PingCLI, *kong.Context, error) {
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
	statsWriter := &TraceStatsBuilder{}
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

		pingRequest := &pkgbot.PingRequestDescriptor{
			PreferV4:     pingCLI.IPv4,
			PreferV6:     pingCLI.IPv6,
			Sources:      []string{locationCode},
			Destinations: []string{destination},
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
	statsWriter := &TraceStatsBuilder{}

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

	pingRequest := &pkgbot.PingRequestDescriptor{
		PreferV4:     pingCLI.IPv4,
		PreferV6:     pingCLI.IPv6,
		Sources:      []string{activeLocationCode},
		Destinations: []string{destination},
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

// PeerStats holds statistics and events for a single peer (IP address) at a hop
type PeerStats struct {
	Peer          string
	PeerRDNS      string
	ASN           string
	ISP           string
	City          string
	CountryAlpha2 string
	Events        []pkgbot.PingEvent // sorted by seq

	// Calculated stats
	ReceivedCount int
	LossCount     int
	MinRTT        int
	MaxRTT        int
	TotalRTT      int
}

// AvgRTT returns the average RTT for this peer
func (ps *PeerStats) AvgRTT() int {
	if ps.ReceivedCount == 0 {
		return 0
	}
	return ps.TotalRTT / ps.ReceivedCount
}

// HopGroup holds statistics for a single hop (TTL level)
type HopGroup struct {
	TTL       int
	Peers     map[string]*PeerStats // keyed by peer IP address
	PeerOrder []string              // order of peers for consistent output
}

// TraceStats holds the complete traceroute statistics
type TraceStats struct {
	Hops     map[int]*HopGroup // keyed by OriginTTL
	HopOrder []int             // sorted order of TTLs for output
}

// TraceStatsBuilder builds TraceStats from ping events
type TraceStatsBuilder struct {
	stats *TraceStats
}

// NewTraceStatsBuilder creates a new TraceStatsBuilder
func NewTraceStatsBuilder() *TraceStatsBuilder {
	return &TraceStatsBuilder{
		stats: &TraceStats{
			Hops: make(map[int]*HopGroup),
		},
	}
}

// WriteEvent adds a ping event to the traceroute statistics
func (statsBuilder *TraceStatsBuilder) WriteEvent(ev pkgbot.PingEvent) {
	stats := statsBuilder.stats

	// Get or create hop group
	hopTTL := ev.OriginTTL
	if hopTTL <= 0 {
		// Fallback for events without OriginTTL (shouldn't happen in traceroute)
		hopTTL = 1
	}

	hop, exists := stats.Hops[hopTTL]
	if !exists {
		hop = &HopGroup{
			TTL:       hopTTL,
			Peers:     make(map[string]*PeerStats),
			PeerOrder: []string{},
		}
		stats.Hops[hopTTL] = hop
		// Update hop order
		stats.HopOrder = append(stats.HopOrder, hopTTL)
		// Keep hops sorted by TTL
		sort.Ints(stats.HopOrder)
	}

	// Determine peer key (use "*" for timeouts)
	peerKey := ev.Peer
	if ev.Timeout || peerKey == "" {
		peerKey = "*"
	}

	// Get or create peer stats
	peerStats, exists := hop.Peers[peerKey]
	if !exists {
		peerStats = &PeerStats{
			Peer:   ev.Peer,
			Events: []pkgbot.PingEvent{},
			MinRTT: -1, // -1 indicates not set yet
			MaxRTT: -1,
		}
		hop.Peers[peerKey] = peerStats
		hop.PeerOrder = append(hop.PeerOrder, peerKey)
	}

	// Add event to peer stats
	peerStats.Events = append(peerStats.Events, ev)

	// Update peer metadata (use latest non-timeout event's data)
	if !ev.Timeout {
		// Update stats
		peerStats.ReceivedCount++
		peerStats.TotalRTT += ev.RTTMs

		if peerStats.MinRTT == -1 || ev.RTTMs < peerStats.MinRTT {
			peerStats.MinRTT = ev.RTTMs
		}
		if peerStats.MaxRTT == -1 || ev.RTTMs > peerStats.MaxRTT {
			peerStats.MaxRTT = ev.RTTMs
		}

		// Update metadata
		if ev.PeerRDNS != "" {
			peerStats.PeerRDNS = ev.PeerRDNS
		}
		if ev.ASN != "" {
			peerStats.ASN = ev.ASN
		}
		if ev.ISP != "" {
			peerStats.ISP = ev.ISP
		}
		if ev.City != "" {
			peerStats.City = ev.City
		}
		if ev.CountryAlpha2 != "" {
			peerStats.CountryAlpha2 = ev.CountryAlpha2
		}
	} else {
		peerStats.LossCount++
	}

	// Sort events by seq
	sort.Slice(peerStats.Events, func(i, j int) bool {
		return peerStats.Events[i].Seq < peerStats.Events[j].Seq
	})

	// Sort PeerOrder by max seq (descending) - peer with latest packets first
	sort.Slice(hop.PeerOrder, func(i, j int) bool {
		peerI := hop.Peers[hop.PeerOrder[i]]
		peerJ := hop.Peers[hop.PeerOrder[j]]
		if peerI == nil || len(peerI.Events) == 0 {
			return false
		}
		if peerJ == nil || len(peerJ.Events) == 0 {
			return true
		}
		// Max seq is the last event (events are already sorted by seq)
		maxSeqI := peerI.Events[len(peerI.Events)-1].Seq
		maxSeqJ := peerJ.Events[len(peerJ.Events)-1].Seq
		return maxSeqI > maxSeqJ // descending order
	})

	// If this is the last hop, delete all higher hops
	if ev.LastHop {
		newHopOrder := make([]int, 0, hopTTL)
		for _, ttl := range stats.HopOrder {
			if ttl <= hopTTL {
				newHopOrder = append(newHopOrder, ttl)
			} else {
				// Delete the hop from the map
				delete(stats.Hops, ttl)
			}
		}
		stats.HopOrder = newHopOrder
	}
}

// GetTraceStats returns the current traceroute statistics
func (statsBuilder *TraceStatsBuilder) GetTraceStats() *TraceStats {
	return statsBuilder.stats
}

// GetHumanReadableText returns a formatted traceroute report
func (statsBuilder *TraceStatsBuilder) GetHumanReadableText() string {
	stats := statsBuilder.stats
	if len(stats.HopOrder) == 0 {
		return "No traceroute data available"
	}

	var sb strings.Builder

	for _, hopTTL := range stats.HopOrder {
		hop := stats.Hops[hopTTL]
		if hop == nil {
			continue
		}

		// First peer gets the hop number
		isFirstPeer := true
		for _, peerKey := range hop.PeerOrder {
			peerStats := hop.Peers[peerKey]
			if peerStats == nil {
				continue
			}

			if isFirstPeer {
				sb.WriteString(fmt.Sprintf("%2d  ", hopTTL))
				isFirstPeer = false
			} else {
				sb.WriteString("    ") // indent for additional peers at same hop
			}

			// Format peer address
			if peerStats.Peer == "" || peerStats.Peer == "*" {
				sb.WriteString("*")
			} else {
				if peerStats.PeerRDNS != "" {
					sb.WriteString(fmt.Sprintf("%s (%s)", peerStats.PeerRDNS, peerStats.Peer))
				} else {
					sb.WriteString(peerStats.Peer)
				}
			}

			// Add location info if available
			if peerStats.City != "" || peerStats.CountryAlpha2 != "" {
				sb.WriteString(" [")
				if peerStats.City != "" {
					sb.WriteString(peerStats.City)
				}
				if peerStats.City != "" && peerStats.CountryAlpha2 != "" {
					sb.WriteString(", ")
				}
				if peerStats.CountryAlpha2 != "" {
					sb.WriteString(peerStats.CountryAlpha2)
				}
				sb.WriteString("]")
			}

			// Add ASN/ISP info if available
			if peerStats.ASN != "" || peerStats.ISP != "" {
				sb.WriteString(" (")
				if peerStats.ASN != "" {
					sb.WriteString(peerStats.ASN)
				}
				if peerStats.ASN != "" && peerStats.ISP != "" {
					sb.WriteString(", ")
				}
				if peerStats.ISP != "" {
					sb.WriteString(peerStats.ISP)
				}
				sb.WriteString(")")
			}

			// Format RTT values
			if len(peerStats.Events) > 0 {
				sb.WriteString("  ")
				for i, ev := range peerStats.Events {
					if i > 0 {
						sb.WriteString("  ")
					}
					if ev.Timeout {
						sb.WriteString("*")
					} else {
						sb.WriteString(fmt.Sprintf("%d ms", ev.RTTMs))
					}
				}
			}

			sb.WriteString("\n")
		}
	}

	return sb.String()
}
