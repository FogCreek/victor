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

type UserChangedEvent struct {
	User            chat.User
	OldName         string
	OldEmailAddress string
}

const changeFmt = `"%s" --> "%s"`

func (u *UserChangedEvent) String() string {
	if len(u.OldEmailAddress) > 0 && len(u.OldName) > 0 {
		return fmt.Sprintf("User %s changed: email "+changeFmt+
			" & name "+changeFmt,
			u.User.ID(), u.OldEmailAddress, u.User.EmailAddress(),
			u.OldName, u.User.Name())
	} else if len(u.OldName) > 0 {
		return fmt.Sprintf("User %s changed: name "+changeFmt,
			u.User.ID(), u.OldName, u.User.Name())
	} else if len(u.OldEmailAddress) > 0 {
		return fmt.Sprintf("User %s changed: name "+changeFmt,
			u.User.ID(), u.OldEmailAddress, u.User.EmailAddress())
	} else {
		return fmt.Sprintf("User %d did not change", u.User.ID())
	}
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

type ChannelChangedEvent struct {
	Channel chat.Channel
	OldName string
}

func (c *ChannelChangedEvent) String() string {
	return fmt.Sprintf("Channel %s has changed from \"%s\" to \"%s\"",
		c.Channel.ID(), c.OldName, c.Channel.Name())
}
