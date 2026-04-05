package bot

import (
	"fmt"
)

// A pair of ask and answer is called a conversation
type ConversationKey struct {
	ChatId int64
	FromId int64
	MsgId  int
}

func (cvKey *ConversationKey) String() string {
	if cvKey == nil {
		return ""
	}
	return fmt.Sprintf("chatId=%v:fromId=%v:msgId=%v", cvKey.ChatId, cvKey.FromId, cvKey.MsgId)
}

type LocationDescriptor struct {
	Id                string
	Label             string
	Alpha2CountryCode string
	CityIATACode      string

	// This field is optional and implementation-specific
	ExtendedAttributes map[string]string
}
