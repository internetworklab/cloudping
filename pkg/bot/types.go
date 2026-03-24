package bot

import (
	"context"
	"fmt"
)

// Ping event data
type PingEvent struct {
	Seq          int
	RTTMs        int
	Peer         string
	PeerRDNS     string
	IPPacketSize int
	Timeout      bool
}

type PingEventsProvider interface {
	GetEventsByLocationCodeAndDestination(ctx context.Context, locationCode string, destination string) <-chan PingEvent
	GetAllLocations(ctx context.Context) []LocationDescriptor
}

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
