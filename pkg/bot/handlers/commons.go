package handlers

import (
	"context"
	"log"

	pkgbot "example.com/rbmq-demo/pkg/bot"
	pkgutils "example.com/rbmq-demo/pkg/utils"
	"github.com/go-telegram/bot/models"
)

type CtxKey string

const (
	CtxKeyJWTSecret           = CtxKey("jwt_secret")
	CtxKeyIssuerName          = CtxKey("issuer_name")
	CtxKeyTxtStreamIntv       = CtxKey("txt_stream_intv")
	CtxKeyTGBtnLayoutCol      = CtxKey("tg_btn_layout_col")
	CtxKeyPingEVProvider      = CtxKey("ping_ev_provider")
	CtxKeyConversationManager = CtxKey("conv_mng")
)

// GetLocationButtons returns an inline keyboard markup with location buttons,
// showing a checkmark indicator on the currently selected location.
func GetLocationButtons(ctx context.Context, selectedLocationCode string, provider pkgbot.PingEventsProvider, numCols int) *models.InlineKeyboardMarkup {
	allLocations, err := provider.GetAllLocations(ctx)
	if err != nil {
		log.Printf("can't get locations: %s", err.Error())
	}
	buttons := make([][]models.InlineKeyboardButton, 0)

	// Create buttons and organize them into rows with numCols buttons each
	for i, loc := range allLocations {
		// Start a new row if we're at the beginning or at a column boundary
		if i%numCols == 0 {
			buttons = append(buttons, make([]models.InlineKeyboardButton, 0))
		}

		// Add button to the current row
		currentRow := &buttons[len(buttons)-1]
		*currentRow = append(*currentRow, models.InlineKeyboardButton{
			Text: getLocationButtonText(loc, selectedLocationCode), CallbackData: pkgbot.FormatPingCallbackData(loc.Id),
		})
	}

	return &models.InlineKeyboardMarkup{InlineKeyboard: buttons}
}

// getLocationButtonText returns the button text for a ping task, with a checkmark if selected.
func getLocationButtonText(loc pkgbot.LocationDescriptor, activeLocationCode string) string {
	activationMark := ""
	if loc.Id == activeLocationCode {
		activationMark = " ✓"
	}
	return pkgutils.Alpha2CountryCodeToUnicode(loc.Alpha2CountryCode) + loc.Label + activationMark
}

func getSubjectFromMessage(message *models.Message) string {
	if message.Chat.Type == models.ChatTypePrivate {
		return message.Chat.Username
	} else if message.Chat.Type == models.ChatTypeGroup || message.Chat.Type == models.ChatTypeSupergroup {
		return message.Chat.Title
	}
	return ""
}
