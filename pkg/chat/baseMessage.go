package chat

// BaseMessage provides a bare set/get implementation of the chat.Message
// interface that can be used by an adapter if it requires no additional logic
// in its Messages.
type BaseMessage struct {
	MsgUser        User
	MsgChannel     Channel
	MsgText        string
	MsgIsDirect    bool
	MsgArchiveLink string
	MsgTimestamp   string
}

// User gets the message's user.
func (m *BaseMessage) User() User {
	return m.MsgUser
}

// Channel gets the message's channel object.
func (m *BaseMessage) Channel() Channel {
	return m.MsgChannel
}

// Text gets the channel's text.
func (m *BaseMessage) Text() string {
	return m.MsgText
}

// IsDirectMessage returns true if the message was direct (private) and false
// otherwise.
func (m *BaseMessage) IsDirectMessage() bool {
	return m.MsgIsDirect
}

// ArchiveLink gets the message's archive link.
func (m *BaseMessage) ArchiveLink() string {
	return m.MsgArchiveLink
}

// Timestamp gets the message's timestamp.
func (m *BaseMessage) Timestamp() string {
	return m.MsgTimestamp
}
