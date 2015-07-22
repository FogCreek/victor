package victor

import (
	"github.com/FogCreek/victor/pkg/chat"
)

// Handler defines an interface for a message handler
type Handler interface {
	Handle(State)
}

// HandlerFunc defines the parameters for a handler's function
type HandlerFunc func(State)

// Handle calls the handler's response function which replies to a message
// appropriately.
func (f HandlerFunc) Handle(s State) {
	f(s)
}

// State defines an interface to provide a handler all of the necessary
// information to reply to a message
type State interface {
	Robot() Robot
	Chat() chat.Adapter
	Message() chat.Message
	Fields() []string
	Reply(string)
}

type state struct {
	robot   Robot
	message chat.Message
	fields  []string
}

// Reply is a convience method to reply to the current message.
//
// Calling "state.Reply(msg) is equivalent to calling
// "state.Chat().Send(state.Message().Channel().ID(), msg)"
func (s *state) Reply(msg string) {
	s.robot.Chat().Send(s.message.Channel().ID(), msg)
}

// Returns the Robot
func (s *state) Robot() Robot {
	return s.robot
}

// Returns the Chat adapter
func (s *state) Chat() chat.Adapter {
	return s.robot.Chat()
}

// Returns the Message
func (s *state) Message() chat.Message {
	return s.message
}

func (s *state) Fields() []string {
	return s.fields
}
