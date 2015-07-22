package definedEvents

import (
	"fmt"

	"github.com/FogCreek/victor/pkg/chat"
)

type ConnectingEvent struct{}

func (c *ConnectingEvent) String() string {
	return "Connecting"
}

type ConnectedEvent struct{}

func (c *ConnectedEvent) String() string {
	return "Connected"
}

type UserEvent struct {
	User       chat.User
	WasRemoved bool
}

func (u *UserEvent) String() string {
	userPart := fmt.Sprintf("User %s (%s) was ", u.User.Name(), u.User.ID())
	if u.WasRemoved {
		return userPart + "removed"
	}
	return userPart + "added"
}

type ChannelEvent struct {
	Channel    chat.Channel
	WasRemoved bool
}

func (c *ChannelEvent) String() string {
	channelPart := fmt.Sprintf("Channel %s (%s) was ", c.Channel.ID(), c.Channel.Name())
	if c.WasRemoved {
		return channelPart + "removed"
	}
	return channelPart + "added"
}
