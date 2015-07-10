package victor

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/FogCreek/victor/pkg/chat"
	"github.com/FogCreek/victor/pkg/events"
	// Blank import used init adapters which registers them with victor
	_ "github.com/FogCreek/victor/pkg/chat/shell"
	_ "github.com/FogCreek/victor/pkg/chat/slackRealtime"
	"github.com/FogCreek/victor/pkg/store"
	// Blank import used init adapters which registers them with victor
	_ "github.com/FogCreek/victor/pkg/store/boltstore"
	_ "github.com/FogCreek/victor/pkg/store/memory"
)

// Robot provides an interface for a victor chat robot.
type Robot interface {
	Run()
	Stop()
	Name() string
	HandleCommand(HandlerDocPair)
	HandleCommandPattern(string, HandlerDocPair)
	HandleCommandRegexp(*regexp.Regexp, HandlerDocPair)
	HandlePattern(string, HandlerFunc)
	HandleRegexp(*regexp.Regexp, HandlerFunc)
	SetDefaultHandler(HandlerFunc)
	EnableHelpCommand()
	Commands() map[string]HandlerDocPair
	Receive(chat.Message)
	Chat() chat.Adapter
	Store() store.Adapter
	AdapterConfig() (interface{}, bool)
	StoreConfig() (interface{}, bool)
	ChatErrors() chan events.ErrorEvent
}

// Config provides all of the configuration parameters needed in order to
// initialize a robot. It also allows for optional configuration structs for
// both the chat and storage adapters which they may or may not require.
type Config struct {
	Name,
	ChatAdapter,
	StoreAdapter string
	AdapterConfig,
	StoreConfig interface{}
}

type robot struct {
	*dispatch
	name     string
	store    store.Adapter
	chat     chat.Adapter
	incoming chan chat.Message
	stop     chan struct{}
	adapterConfig,
	storeConfig interface{}
	chatErrorChannel chan events.ErrorEvent
}

// New returns a robot
func New(config Config) *robot {
	chatAdapter := config.ChatAdapter
	if chatAdapter == "" {
		log.Println("Shell adapter has been removed.")
		chatAdapter = "shell"
	}

	chatInitFunc, err := chat.Load(config.ChatAdapter)

	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	storeAdapter := config.StoreAdapter
	if storeAdapter == "" {
		storeAdapter = "memory"
	}

	storeInitFunc, err := store.Load(storeAdapter)

	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	botName := config.Name
	if botName == "" {
		botName = "victor"
	}

	bot := &robot{
		name:     botName,
		incoming: make(chan chat.Message),
		stop:     make(chan struct{}),
	}

	bot.chatErrorChannel = make(chan events.ErrorEvent)
	bot.store = storeInitFunc(bot)
	bot.adapterConfig = config.AdapterConfig
	bot.dispatch = newDispatch(bot)
	bot.chat = chatInitFunc(bot)
	return bot
}

// Receive accepts messages for processing
func (r *robot) Receive(m chat.Message) {
	r.incoming <- m
}

// Run starts the robot.
func (r *robot) Run() {
	r.chat.Run()
	go func() {
		for {
			select {
			case <-r.stop:
				close(r.incoming)
				return
			case m := <-r.incoming:
				if strings.ToLower(m.User().Name()) != r.name {
					go r.ProcessMessage(m)
				}
			}
		}
	}()
}

// Stop shuts down the bot
func (r *robot) Stop() {
	r.chat.Stop()
	close(r.stop)
	close(r.chatErrorChannel)
}

// Name returns the name of the bot
func (r *robot) Name() string {
	return r.name
}

// Store returns the data store adapter
func (r *robot) Store() store.Adapter {
	return r.store
}

// Chat returns the chat adapter
func (r *robot) Chat() chat.Adapter {
	return r.chat
}

func (r *robot) AdapterConfig() (interface{}, bool) {
	return r.adapterConfig, r.adapterConfig != nil
}

func (r *robot) StoreConfig() (interface{}, bool) {
	return r.storeConfig, r.storeConfig != nil
}

func (r *robot) ChatErrors() chan events.ErrorEvent {
	return r.chatErrorChannel
}

// OnlyAllow provides a way of permitting specific users
// to execute a handler registered with the bot
func OnlyAllow(userNames []string, action func(s State)) func(State) {
	return func(s State) {
		actual := s.Message().User().Name()
		for _, name := range userNames {
			if name == actual {
				action(s)
				return
			}
		}

		s.Chat().Send(s.Message().ChannelID(), fmt.Sprintf("Sorry, %s. I can't let you do that.", actual))
	}
}
