package mockAdapter

import (
	"strconv"

	"github.com/FogCreek/victor/pkg/chat"
)

var nextID = 0

func init() {
	chat.Register("mockAdapter", func(r chat.Robot) chat.Adapter {
		id := nextID
		nextID++
		return &MockChatAdapter{
			id: strconv.Itoa(id),
		}
	})
}

type MockChatAdapter struct {
	id string
}

func (m *MockChatAdapter) Run() error {
	return nil
}

func (m *MockChatAdapter) Send(string, string) {
	return
}

func (m *MockChatAdapter) SendDirectMessage(string, string) {
	return
}

func (m *MockChatAdapter) Stop() {
	return
}

func (m *MockChatAdapter) ID() string {
	return m.id
}

func (m *MockChatAdapter) GetUser(string) chat.User {
	return nil
}

func (m *MockChatAdapter) IsPotentialUser(string) bool {
	return false
}
