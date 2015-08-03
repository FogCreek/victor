package chat

import (
	"fmt"

	"github.com/FogCreek/victor/pkg/events"
	"github.com/FogCreek/victor/pkg/store"
)

var adapters = map[string]InitFunc{}

func Register(name string, init InitFunc) {
	adapters[name] = init
}

func Load(name string) (InitFunc, error) {
	a, ok := adapters[name]

	if !ok {
		return nil, fmt.Errorf("unkown adapter: %s", name)
	}

	return a, nil
}

type InitFunc func(Robot) Adapter

type Adapter interface {
	Run()
	Send(string, string)
	SendDirectMessage(string, string)
	SendTyping(string)
	Stop()
	// ID should return a unique ID for that adapter which is guarenteed to
	// remain constant as long as the adapter points to the same chat instance.
	ID() string
	GetUser(string) User
	GetChannel(string) Channel
	IsPotentialUser(string) bool
	IsPotentialChannel(string) bool
	GetAllUsers() []User
	GetPublicChannels() []Channel

	// Name returns the name of the team/chat instance.
	Name() string
	MaxLength() int
}

type Robot interface {
	Name() string
	Store() store.Adapter
	Chat() Adapter
	Receive(Message)
	AdapterConfig() (interface{}, bool)
	ChatErrors() chan events.ErrorEvent
	ChatEvents() chan events.ChatEvent
}

type Message interface {
	User() User
	Channel() Channel
	Text() string
	IsDirectMessage() bool
	ArchiveLink() string
	Timestamp() string
}

type User interface {
	ID() string
	Name() string
	EmailAddress() string
	IsBot() bool
}

type Channel interface {
	Name() string
	ID() string
}
