package mockAdapter

import (
	"strconv"
	"sync"

	"github.com/FogCreek/victor/pkg/chat"
)

var (
	nextID         = 0
	nextIDMutex    = &sync.Mutex{}
	defaultUserRet = &chat.BaseUser{
		UserName:  "Fake User",
		UserID:    "UFakeUser",
		UserIsBot: false,
		UserEmail: "fake@example.com",
	}
	defaultChannelRet = &chat.BaseChannel{
		ChannelName: "Fake Channel",
		ChannelID:   "CFakeChannel",
	}
)

// init Registers the mockAdapter with the victor framework under the chat
// adapter name "mockAdapter".
func init() {
	chat.Register("mockAdapter", func(r chat.Robot) chat.Adapter {
		nextIDMutex.Lock()
		id := nextID
		nextID++
		nextIDMutex.Unlock()
		return &MockChatAdapter{
			robot:                 r,
			id:                    strconv.Itoa(id),
			Sent:                  make([]MockMessagePair, 0, 10),
			SentPublic:            make([]MockMessagePair, 0, 10),
			SentDirect:            make([]MockMessagePair, 0, 10),
			IsPotentialUserRet:    true,
			IsPotentialChannelRet: true,
			UserRet:               defaultUserRet,
			AllUsersRet:           []chat.User{defaultUserRet},
			PublicChannelsRet:     []chat.Channel{defaultChannelRet},
		}
	})
}

// MockChatAdapter provides an "empty" chat adapter which can be used to test
// victor and/or victor handler functions. It stores all sent messages to an
// exported array and allows certain function's returned values (GetUser,
// IsPotentialUser, etc.) to be set.
type MockChatAdapter struct {
	id    string
	robot chat.Robot
	Sent,
	SentPublic,
	SentDirect []MockMessagePair
	UserRet               chat.User
	AllUsersRet           []chat.User
	PublicChannelsRet     []chat.Channel
	IsPotentialUserRet    bool
	IsPotentialChannelRet bool
}

// Clear clears the contents of the "Sent" array.
func (m *MockChatAdapter) Clear() {
	m.Sent = make([]MockMessagePair, 0, 10)
	m.SentPublic = make([]MockMessagePair, 0, 10)
	m.SentDirect = make([]MockMessagePair, 0, 10)
}

// Receive mocks a message being received by the chat adapter.
func (m *MockChatAdapter) Receive(mp chat.Message) {
	m.robot.Receive(mp)
}

// Run does nothing as the mockAdapter does not connect to anything.
func (m *MockChatAdapter) Run() {
	return
}

// Send stores the given channelID and text to the exported array "Sent" as
// a MockMessagePair.
func (m *MockChatAdapter) Send(channelID, text string) {
	m.Sent = append(m.Sent, MockMessagePair{
		text:      text,
		channelID: channelID,
		isDirect:  false,
	})
	m.SentPublic = append(m.SentPublic, MockMessagePair{
		text:      text,
		channelID: channelID,
		isDirect:  false,
	})
}

// SendDirectMessage stores the given userID and text to the exported array
// "Sent" as a MockMessagePair with the "IsDirect" flag set to true.
func (m *MockChatAdapter) SendDirectMessage(userID, text string) {
	m.Sent = append(m.Sent, MockMessagePair{
		text:     text,
		userID:   userID,
		isDirect: true,
	})
	m.SentDirect = append(m.SentDirect, MockMessagePair{
		text:     text,
		userID:   userID,
		isDirect: true,
	})
}

// SendTyping does nothing.
func (m *MockChatAdapter) SendTyping(channelID string) {
	return
}

// Stop does nothing.
func (m *MockChatAdapter) Stop() {
	return
}

// ID returns the mockAdapter's ID.
func (m *MockChatAdapter) ID() string {
	return m.id
}

// GetUser returns the mockAdapter's set "UserRet" property which can be
// set to any chat.User instance (default value has full name "Fake User").
func (m *MockChatAdapter) GetUser(string) chat.User {
	return m.UserRet
}

func (m *MockChatAdapter) GetAllUsers() []chat.User {
	return m.AllUsersRet
}

func (m *MockChatAdapter) GetPublicChannels() []chat.Channel {
	return m.PublicChannelsRet
}

// IsPotentialUser returns the mockAdapter's set "IsPotentialUserRet" property
// which has a default value of "true".
func (m *MockChatAdapter) IsPotentialUser(string) bool {
	return m.IsPotentialUserRet
}

func (m *MockChatAdapter) IsPotentialChannel(string) bool {
	return m.IsPotentialChannelRet
}

// MockMessagePair is used to store messages that are sent by chat handlers to
// the mockAdapter instance.
type MockMessagePair struct {
	channelID,
	userID,
	text string
	isDirect bool
}

// ChannelID returns the id of the channel that the sent message was intended
// for. If this is a direct message then this will not be set.
func (mp *MockMessagePair) ChannelID() string {
	return mp.channelID
}

// Text returns the message's full text.
func (mp *MockMessagePair) Text() string {
	return mp.text
}

// UserID returns the id of the user that the message was intended for. This
// will not be sent unless it is a direct message.
func (mp *MockMessagePair) UserID() string {
	return mp.userID
}

// IsDirect returns true if the message is direct and false otherwise. The
// output of this can be used to determine whether ChannelID() or UserID are
// relavent to this message.
func (mp *MockMessagePair) IsDirect() bool {
	return mp.isDirect
}
