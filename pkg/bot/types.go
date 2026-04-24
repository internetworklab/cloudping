package bot

import (
	"fmt"
)

// A pair of ask and answer is called a conversation
// we use the (chat_id:msg_id) tuple as the key for a conversation,
// which uniquely identifies a message globally,
// and that message is also the second message of the conversation (the first reply).
type ConversationKey struct {
	ChatId int64
	MsgId  int
}

func (cvKey *ConversationKey) String() string {
	if cvKey == nil {
		return ""
	}
	return fmt.Sprintf("chatId=%v:msgId=%v", cvKey.ChatId, cvKey.MsgId)
}
