package events

import "fmt"

type ChatEvent interface {
	fmt.Stringer
}

type BaseChatEvent struct {
	Text string
}

func (bc *BaseChatEvent) String() string {
	return bc.Text
}
