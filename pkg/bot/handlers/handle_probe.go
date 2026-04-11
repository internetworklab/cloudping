package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"slices"

	"github.com/alecthomas/kong"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	pkgbitmap "github.com/internetworklab/cloudping/pkg/bitmap"
	pkgbot "github.com/internetworklab/cloudping/pkg/bot"
	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
)

const defaultGridCellSize = 32

type ProbeCLI struct {
	From string `short:"s" name:"from" help:"Specify the source node for originating packets"`
	CIDR string `arg:"" name:"cidr" help:"CIDR of the subnet to probe, e.g. 172.23.0.0/24"`
}

type ProbeHandler struct {
	// Name of fonts to search
	FontNames []string

	EventsProvider pkgbot.PingEventsProvider
}

func (handler *ProbeHandler) getEVsProvider() (pkgbot.PingEventsProvider, error) {
	if handler.EventsProvider == nil {
		return nil, errors.New("PingEventsProvider is not provided")
	}
	return handler.EventsProvider, nil
}

func (handler *ProbeHandler) GetUsage() string {
	return "/probe -s=<source_node_id> <cidr>"
}

func (handler *ProbeHandler) parseCLIString(cliString string) (*ProbeCLI, *kong.Context, error) {

	cliSegs := pkgutils.SplitBySpace(cliString)
	if len(cliSegs) == 0 {
		return nil, nil, errors.New("no arguments provided")
	}

	pingCLI := new(ProbeCLI)
	parser, err := kong.New(pingCLI)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create kong parser: %w", err)
	}

	kongCtx, err := parser.Parse(cliSegs)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse CLI: %w", err)
	}

	return pingCLI, kongCtx, nil
}

func (handler *ProbeHandler) getFontNames() []string {
	var defaultFontNames = []string{"Noto Sans Mono", "monospace"}
	if handler == nil || len(handler.FontNames) == 0 {
		return defaultFontNames
	}
	return handler.FontNames
}

func (handler *ProbeHandler) HandleProbe(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatId := update.Message.Chat.ID
	msgId := update.Message.ID
	replyParams := &models.ReplyParameters{ChatID: chatId, MessageID: msgId}

	provider, err := handler.getEVsProvider()
	if err != nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          chatId,
			Text:            fmt.Sprintf("Can't get location provider: %s", err.Error()),
			ReplyParameters: replyParams,
		})
		return
	}

	allLocs, err := provider.GetAllLocations(ctx)
	if err != nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          chatId,
			Text:            fmt.Sprintf("Can't get locations: %s", err.Error()),
			ReplyParameters: replyParams,
		})
		return
	}

	cliString := pkgbot.TrimCommandPrefix(update.Message.Text)
	probeCLI, _, err := handler.parseCLIString(cliString)
	if err != nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          chatId,
			Text:            fmt.Sprintf("Error: %s\nUsage: %s", err.Error(), handler.GetUsage()),
			ReplyParameters: replyParams,
		})
		return
	}

	idx := slices.IndexFunc(allLocs, func(elem pkgbot.LocationDescriptor) bool {
		return elem.Id == probeCLI.From
	})
	if idx == -1 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          chatId,
			Text:            fmt.Sprintf("Empty or invalid source node %q.\nSee /list for available nodes and specify the source node with --from=<node_id>", probeCLI.From),
			ReplyParameters: replyParams,
		})
		return
	}

	rttMs := make([]int, 0) // todo

	imgFilename, err := pkgbitmap.GenerateRandomRGBAPNGBitmap(
		rttMs,
		defaultGridCellSize,
		probeCLI.CIDR,
		handler.getFontNames(),
	)

	if err != nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          chatId,
			Text:            err.Error(),
			ReplyParameters: replyParams,
		})
		return
	}
	defer os.Remove(imgFilename)
	imgFile, err := os.Open(imgFilename)
	if err != nil {
		log.Panic(err)
	}
	defer imgFile.Close()
	imgFileUp := models.InputFileUpload{Filename: imgFilename, Data: imgFile}

	_, err = b.SendPhoto(ctx, &bot.SendPhotoParams{
		ChatID:          chatId,
		Photo:           &imgFileUp,
		ReplyParameters: replyParams,
	})
	if err != nil {
		log.Printf("failed to send probe response: %v", err)
	}
}
