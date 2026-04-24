package handlers

import (
	"context"
	"log"

	"github.com/go-telegram/bot/models"
	pkgtui "github.com/internetworklab/cloudping/pkg/tui"
	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
)

const DefaultBtnLayoutCols = 4

// GetLocationButtons returns an inline keyboard markup with location buttons,
// showing a checkmark indicator on the currently selected location.
func GetLocationButtons(ctx context.Context, selectedLocationCode string, provider pkgtui.PingEventsProvider, numCols int, callbackQueryFormatter func(loc pkgtui.LocationDescriptor) string) *models.InlineKeyboardMarkup {
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
			Text: getLocationButtonText(loc, selectedLocationCode), CallbackData: callbackQueryFormatter(loc),
		})
	}

	return &models.InlineKeyboardMarkup{InlineKeyboard: buttons}
}

// getLocationButtonText returns the button text for a ping task, with a checkmark if selected.
func getLocationButtonText(loc pkgtui.LocationDescriptor, activeLocationCode string) string {
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
