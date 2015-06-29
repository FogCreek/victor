package chat

import (
	"fmt"

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
	Run() error
	Send(string, string)
	SendDirectMessage(string, string)
	Stop()
	// ID should return a unique ID for that adapter which is guarenteed to
	// remain constant as long as the adapter points to the same chat instance.
	ID() string
	GetUser(string) User
	IsPotentialUser(string) bool
}

type Robot interface {
	Name() string
	Store() store.Adapter
	Chat() Adapter
	Receive(Message)
	AdapterConfig() (interface{}, bool)
}

type Message interface {
	User() User
	ChannelID() string
	ChannelName() string
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
