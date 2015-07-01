package mockState

import (
	"github.com/FogCreek/victor"
	"github.com/FogCreek/victor/pkg/chat"
)

type MockState struct {
	MockRobot   victor.Robot
	MockMessage *chat.BaseMessage
	MockFields  []string
}

// Robot returns the Robot.
func (s *MockState) Robot() victor.Robot {
	return s.MockRobot
}

// Chat returns the Chat adapter.
func (s *MockState) Chat() chat.Adapter {
	return s.MockRobot.Chat()
}

// Message returns the Message.
func (s *MockState) Message() chat.Message {
	return s.MockMessage
}

// Fields returns the Message's Fields.
func (s *MockState) Fields() []string {
	return s.MockFields
}
