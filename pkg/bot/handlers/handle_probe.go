package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
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

	LocationsProvider pkgbot.LocationsProvider

	ProbeEventsProvider pkgbot.ProbeEventsProvider

	MaxBitsizeAllowed *int
}

func (handler *ProbeHandler) getMaxBitsize() int {
	if x := handler.MaxBitsizeAllowed; x != nil {
		return *x
	}
	const defaultMaxBitSize = 10
	return defaultMaxBitSize
}

func (handler *ProbeHandler) getLocationsProvider() (pkgbot.LocationsProvider, error) {
	if handler.LocationsProvider == nil {
		return nil, errors.New("Locations provider is not provided")
	}
	return handler.LocationsProvider, nil
}

func (handler *ProbeHandler) getEVsProvider() (pkgbot.ProbeEventsProvider, error) {
	if handler.ProbeEventsProvider == nil {
		return nil, errors.New("Events provider is not provided")
	}
	return handler.ProbeEventsProvider, nil
}

func (handler *ProbeHandler) GetUsage() string {
	return "/probe -s <source_node_id> <cidr>"
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

	locsProvider, err := handler.getLocationsProvider()
	if err != nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          chatId,
			Text:            fmt.Sprintf("Can't get location provider: %s", err.Error()),
			ReplyParameters: replyParams,
		})
		return
	}

	allLocs, err := locsProvider.GetAllLocations(ctx)
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

	_, cidrObj, err := net.ParseCIDR(probeCLI.CIDR)
	if err != nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          chatId,
			Text:            fmt.Sprintf("Failed to parse CIDR %s: %s", probeCLI.CIDR, err.Error()),
			ReplyParameters: replyParams,
		})
		return
	}

	ones, bits := cidrObj.Mask.Size()
	bitSize := bits - ones // number of host bits, ones is the number of bits for network address
	maxBitSize := handler.getMaxBitsize()
	if bitSize > maxBitSize {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          chatId,
			Text:            fmt.Sprintf("CIDR %s is too large, maximum bit size is %d", probeCLI.CIDR, maxBitSize),
			ReplyParameters: replyParams,
		})
		return
	}

	numSamples := uint32(1) << uint32(bitSize)
	rttMs := make([]int, numSamples)
	for i := range rttMs {
		// mocking an all-timeout scenario, except the first idx
		rttMs[i] = -1
	}
	rttMs[len(rttMs)-1] = 1

	imgFilename, err := pkgbitmap.GenerateRandomRGBAPNGBitmap(
		rttMs,
		defaultGridCellSize,
		*cidrObj,
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
