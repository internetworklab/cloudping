package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	pkgbot "github.com/internetworklab/cloudping/pkg/bot"
	pkgtable "github.com/internetworklab/cloudping/pkg/table"
)

type ListHandler struct {
	EventsProvider pkgbot.PingEventsProvider
}

func (prober *ListHandler) getEVsProvider() (pkgbot.PingEventsProvider, error) {
	if prober.EventsProvider == nil {
		return nil, errors.New("PingEventsProvider is not provided")
	}
	return prober.EventsProvider, nil
}

func (prober *ListHandler) HandleList(ctx context.Context, b *bot.Bot, update *models.Update) {
	provider := prober.EventsProvider
	allLocs, err := provider.GetAllLocations(ctx)
	chatId := update.Message.Chat.ID
	msgId := update.Message.ID
	replyParam := &models.ReplyParameters{
		ChatID:    chatId,
		MessageID: msgId,
	}
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

	var nodesTable pkgtable.Table = prober.getExampleTable()
	tableText := nodesTable.GetHumanReadableText(defaultColGap, defaultRowGap, defaultMaxColWidth)
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

func (handler *ListHandler) getExampleTable() pkgtable.Table {
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
