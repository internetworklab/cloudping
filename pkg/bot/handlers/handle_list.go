package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	pkgtable "github.com/internetworklab/cloudping/pkg/table"
	pkgtui "github.com/internetworklab/cloudping/pkg/tui"
	pkgtuirenderer "github.com/internetworklab/cloudping/pkg/tui/renderer"
)

type ListHandler struct {
	LocationsProvider pkgtui.LocationsProvider
}

func (handler *ListHandler) getLocsProvider() (pkgtui.LocationsProvider, error) {
	if handler.LocationsProvider == nil {
		return nil, errors.New("LocationsProvider is not provided")
	}
	return handler.LocationsProvider, nil
}

func (handler *ListHandler) HandleList(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatId := update.Message.Chat.ID
	msgId := update.Message.ID
	replyParam := &models.ReplyParameters{
		ChatID:    chatId,
		MessageID: msgId,
	}

	provider, err := handler.getLocsProvider()
	if err != nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          chatId,
			Text:            fmt.Sprintf("Can't get location provider: %s", err.Error()),
			ReplyParameters: replyParam,
		})
		return
	}

	allLocs, err := provider.GetAllLocations(ctx)
	if err != nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          chatId,
			Text:            fmt.Sprintf("Can't get locations: %s", err.Error()),
			ReplyParameters: replyParam,
		})
		return
	}

	if len(allLocs) == 0 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          chatId,
			Text:            fmt.Sprintf("Can't get locations: %s", "No node is available."),
			ReplyParameters: replyParam,
		})
		return
	}

	renderer := &pkgtuirenderer.LocationsTableRenderer{}
	var nodesTable pkgtable.Table = renderer.Render(allLocs)
	const maxColWidth = 30
	const defaultMaxColWidth int = 24
	const defaultColGap int = 2
	const defaultRowGap int = 0

	tableText := nodesTable.GetHumanReadableText(defaultColGap, defaultRowGap, maxColWidth)
	entities := []models.MessageEntity{
		{
			Type:   models.MessageEntityTypePre,
			Offset: 0,
			Length: len(tableText),
		},
	}
	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          chatId,
		Text:            tableText,
		Entities:        entities,
		ReplyParameters: replyParam,
	})
	if err != nil {
		log.Printf("Failed to send bot message: %s", err.Error())
	}
}
