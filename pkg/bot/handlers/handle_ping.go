package handlers

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	pkgbot "example.com/rbmq-demo/pkg/bot"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

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

func HandlePing(ctx context.Context, b *bot.Bot, update *models.Update) {
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

func HandlePingQueryCallback(ctx context.Context, b *bot.Bot, update *models.Update) {
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
