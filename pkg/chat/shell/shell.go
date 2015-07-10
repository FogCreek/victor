package shell

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/FogCreek/victor/pkg/chat"
	"github.com/FogCreek/victor/pkg/events"
)

const (
	timeFormat  = "20060102150405"
	channelID   = "shell_channel"
	channelName = "shell channel"
)

var (
	realUser = &chat.BaseUser{
		UserID:    "shell_user",
		UserName:  "[Shell User]",
		UserEmail: "user@example.com",
		UserIsBot: false,
	}
	nextID      = 0
	nextIDMutex = &sync.Mutex{}
)

func init() {
	chat.Register("shell", func(r chat.Robot) chat.Adapter {
		nextIDMutex.Lock()
		id := strconv.Itoa(nextID)
		nextID++
		nextIDMutex.Unlock()
		return &Adapter{
			robot: r,
			stop:  make(chan bool),
			id:    id,
			lines: make(chan string),
		}
	})
}

type Adapter struct {
	robot chat.Robot
	stop  chan bool
	id    string
	lines chan string
}

func (a *Adapter) Run() error {
	reader := bufio.NewReader(os.Stdin)

	go func() {
		for {
			if line, _, err := reader.ReadLine(); err == nil {
				a.lines <- string(line)
			} else {
				a.robot.ChatErrors() <- &events.BaseError{
					ErrorObj: err,
				}
			}
		}
	}()
	go a.monitorEvents()
	return nil
}

func (a *Adapter) monitorEvents() {
	for {
		select {
		case <-a.stop:
			return
		case line := <-a.lines:
			a.robot.Receive(&chat.BaseMessage{
				MsgText:        string(line),
				MsgUser:        realUser,
				MsgChannelID:   channelID,
				MsgChannelName: channelName,
				MsgIsDirect:    true,
				MsgArchiveLink: "",
				MsgTimestamp:   time.Now().Format(timeFormat),
			})
		}
	}
}

func (a *Adapter) Send(channelID, msg string) {
	fmt.Println("SEND:", msg)
}

func (a *Adapter) SendDirectMessage(userID, msg string) {
	a.Send("", "DIRECT MESSAGE: "+msg)
}

func (a *Adapter) SendTyping(string) {
	return
}

func (a *Adapter) Stop() {
	a.stop <- true
	close(a.stop)
}

func (a *Adapter) ID() string {
	return a.id
}

func (a *Adapter) GetUser(userID string) chat.User {
	if userID == realUser.ID() {
		return realUser
	}
	return nil
}

func (a *Adapter) IsPotentialUser(userID string) bool {
	return userID == realUser.ID()
}

func (a *Adapter) NormalizeUserID(userID string) string {
	return userID
}