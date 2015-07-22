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

// Reply is a convience method to reply to the current message.
//
// Calling "state.Reply(msg) is equivalent to calling
// "state.Chat().Send(state.Message().Channel().ID(), msg)"
func (s *MockState) Reply(msg string) {
	s.MockRobot.Chat().Send(s.Message().Channel().ID(), msg)
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
