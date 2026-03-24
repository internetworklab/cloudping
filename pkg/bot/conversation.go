package bot

import (
	"context"
	"errors"
	"sync"
	"time"
)

type MessageRecord struct {
	DateTime time.Time
	Content  string
}

type ConversationContext struct {
	Cancaller context.CancelFunc
	Messages  []MessageRecord
}

// ConversationManager allows a new message handler to interrupt and stop
// any currently running handler for the same conversation.
//
// For example, if a user runs `/ping somewhere.com` (handler A starts processing),
// and then immediately presses a button, the button's callback handler (B)
// will cancel handler A and take over the conversation.
type ConversationManager struct {
	store sync.Map
}

func (convMng *ConversationManager) CheckIn(ctx context.Context, key *ConversationKey, initialMessage MessageRecord, canceller context.CancelFunc) error {
	convCtx := &ConversationContext{
		Cancaller: canceller,
		Messages:  []MessageRecord{initialMessage},
	}
	convMng.store.Store(key.String(), convCtx)
	return nil
}

var ErrNonExistConversation = errors.New("Conversation doesn't exist")

func (convMng *ConversationManager) CutIn(ctx context.Context, key *ConversationKey, canceller context.CancelFunc) ([]MessageRecord, error) {
	if convCtxAny, ok := convMng.store.Load(key.String()); ok {
		convCtx := convCtxAny.(*ConversationContext)
		convCtx.Cancaller()
		newConvCtx := &ConversationContext{
			Cancaller: canceller,
			Messages:  make([]MessageRecord, len(convCtx.Messages)),
		}
		copy(newConvCtx.Messages, convCtx.Messages)
		convMng.store.Store(key.String(), newConvCtx)
		return newConvCtx.Messages, nil
	}

	return nil, ErrNonExistConversation
}
