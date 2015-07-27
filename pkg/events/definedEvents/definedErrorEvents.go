package definedEvents

import (
	"errors"
	"fmt"
)

type InvalidAuth struct{}

func (i *InvalidAuth) Error() string {
	return i.ErrorObject().Error()
}

func (i *InvalidAuth) ErrorObject() error {
	return errors.New("Invalid Auth")
}

func (i *InvalidAuth) IsFatal() bool {
	return true
}

type Disconnect struct {
	Intentional bool
}

func (d *Disconnect) Error() string {
	return d.ErrorObject().Error()
}

func (d *Disconnect) IsFatal() bool {
	return false
}

func (d *Disconnect) ErrorObject() error {
	if d.Intentional {
		return errors.New("Intentional Disconnect")
	} else {
		return errors.New("Unexpected Disconnect")
	}
}

type MessageTooLong struct {
	ChannelID string
	Text      string
	MaxLength int
}

func (m *MessageTooLong) Error() string {
	return m.ErrorObject().Error()
}

func (m *MessageTooLong) ErrorObject() error {
	return fmt.Errorf("Message too long (max %d chars)", m.MaxLength)
}

func (m *MessageTooLong) IsFatal() bool {
	return false
}
