package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	pkgbot "github.com/internetworklab/cloudping/pkg/bot"
	pkgnodereg "github.com/internetworklab/cloudping/pkg/nodereg"
	pkgtable "github.com/internetworklab/cloudping/pkg/table"
)

type ListHandler struct {
	EventsProvider pkgbot.PingEventsProvider
}

func (handler *ListHandler) getEVsProvider() (pkgbot.PingEventsProvider, error) {
	if handler.EventsProvider == nil {
		return nil, errors.New("PingEventsProvider is not provided")
	}
	return handler.EventsProvider, nil
}

func (handler *ListHandler) HandleList(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatId := update.Message.Chat.ID
	msgId := update.Message.ID
	replyParam := &models.ReplyParameters{
		ChatID:    chatId,
		MessageID: msgId,
	}

	provider, err := handler.getEVsProvider()
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

	var nodesTable pkgtable.Table = handler.getNodeTable(allLocs)
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

func (handler *ListHandler) getNodeTable(locations []pkgbot.LocationDescriptor) pkgtable.Table {
	// Write header rows
	table := pkgtable.Table{}
	table.Rows = append(
		table.Rows,
		pkgtable.Row{Cells: []string{"NodeId", "Network", "City,Country"}},
		pkgtable.Row{Cells: []string{"", "(Alternative Networks)", "(Location)"}},
		pkgtable.Row{Cells: []string{}},
	)

	for _, loc := range locations {
		nodeName := loc.Id
		isps := make([]string, 0)
		cityContry := ""
		exactLoc := ""
		if locAttrs := loc.ExtendedAttributes; locAttrs != nil {
			if asn, hit := locAttrs[pkgnodereg.AttributeKeyASN]; hit && asn != "" {
				isps = append(isps, asn)
			}
			if isp, hit := locAttrs[pkgnodereg.AttributeKeyISP]; hit && isp != "" {
				if len(isps) > 0 {
					isps[len(isps)-1] = isps[len(isps)-1] + " " + isp
				} else {
					isps = append(isps, isp)
				}
			}
			if asn, hit := locAttrs[pkgnodereg.AttributeKeyDN42ASN]; hit && asn != "" {
				isps = append(isps, asn)
			}
			if isp, hit := locAttrs[pkgnodereg.AttributeKeyDN42ISP]; hit && isp != "" {
				if len(isps) > 0 {
					isps[len(isps)-1] = isps[len(isps)-1] + " " + isp
				} else {
					isps = append(isps, isp)
				}
			}

			if city, hit := locAttrs[pkgnodereg.AttributeKeyCityName]; hit && city != "" {
				cityContry = city
			}
			if countryCode, hit := locAttrs[pkgnodereg.AttributeKeyCountryCode]; hit && countryCode != "" {
				if cityContry == "" {
					cityContry = countryCode
				} else {
					cityContry = cityContry + "," + countryCode
				}
			}

			if l, hit := locAttrs[pkgnodereg.AttributeKeyExactLocation]; hit && l != "" {
				exactLoc = l
			}
		}

		rows := handler.getNodeRows(nodeName, isps, cityContry, exactLoc)
		table.Rows = append(table.Rows, rows...)
		table.Rows = append(table.Rows, pkgtable.Row{Cells: []string{}})
	}

	return table
}

func (handler *ListHandler) getExampleTable() pkgtable.Table {
	// Write header rows
	table := pkgtable.Table{}
	table.Rows = append(
		table.Rows,
		pkgtable.Row{Cells: []string{"NodeId", "Network", "City,Country"}},
		pkgtable.Row{Cells: []string{"", "(Alternative Networks)", "(Location)"}},
		pkgtable.Row{Cells: []string{}},
	)

	rows := handler.getNodeRows("us-lax1", []string{"AS1331 EXAMPLE-LLC", "AS4242421234 EXAMPLE-DN42"}, "LAX,US", "48.1952,16.3503")
	table.Rows = append(table.Rows, rows...)
	table.Rows = append(table.Rows, pkgtable.Row{Cells: []string{}})

	rows = handler.getNodeRows("hk-hkg1", []string{"AS1331 EXAMPLE-LLC", "AS4242421234 EXAMPLE-DN42"}, "HKG,HK", "48.1952,16.3503")
	table.Rows = append(table.Rows, rows...)
	table.Rows = append(table.Rows, pkgtable.Row{Cells: []string{}})

	rows = handler.getNodeRows("jp-nrt1", []string{"AS1331 EXAMPLE-LLC", "AS4242421234 EXAMPLE-DN42"}, "NRT,JP", "48.1952,16.3503")
	table.Rows = append(table.Rows, rows...)
	table.Rows = append(table.Rows, pkgtable.Row{Cells: []string{}})

	return table
}

func (handler *ListHandler) getNodeRows(nodeName string, isps []string, cityCountry string, location string) []pkgtable.Row {
	if nodeName == "" {
		return nil
	}

	nameCol := make([]string, 0)
	nameCol = append(nameCol, nodeName)

	ispCol := make([]string, 0)
	for _, isp := range isps {
		isp = strings.TrimSpace(isp)
		if isp != "" {
			ispCol = append(ispCol, isp)
		}
	}

	locCol := make([]string, 0)
	cityCountry = strings.TrimSpace(cityCountry)
	if cityCountry != "" {
		locCol = append(locCol, cityCountry)
	}

	location = strings.TrimSpace(location)
	if location != "" {
		locCol = append(locCol, location)
	}

	rowHeight := len(nameCol)

	if h := len(ispCol); h > rowHeight {
		rowHeight = h
	}

	if h := len(locCol); h > rowHeight {
		rowHeight = h
	}

	rows := make([]pkgtable.Row, rowHeight)
	for i := range rows {
		rows[i].Cells = make([]string, 3)
		if i < len(nameCol) {
			rows[i].Cells[0] = nameCol[i]
		}
		if i < len(ispCol) {
			rows[i].Cells[1] = ispCol[i]
		}
		if i < len(locCol) {
			rows[i].Cells[2] = locCol[i]
		}
	}

	return rows
}
