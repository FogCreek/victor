package slackRealtime

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/brettbuddin/victor/pkg/chat"
	"github.com/nlopes/slack"
)

// The Slack Websocket's registered adapter name for the victor framework.
const AdapterName = "slackRealtime"

// Prefix for the user's ID which is used when reading/writing from the bot's store
const userInfoPrefix = AdapterName + "."

// Regex that a message should match in order to be considered a potential bot
// command. Needs to be kept up to date with format in
// github.com/brettbuddin/victor/pkg/handler.go in the Direct function.
// TODO store this in only one place...
const botRegexFormat = "(?i)\\A(?:(?:@)?%s[:,]?\\s*|/)"

// channelGroupInfo is used instead of the slack library's Channel struct since we
// are trying to consider channels and groups to be roughly the same while it
// considers them seperate and provides no way to consolidate them on its own.
//
// This also allows us to throw our information that we don't care about (members, etc.).
type channelGroupInfo struct {
	Name      string
	ID        string
	IsDM      bool
	UserID    string
	IsChannel bool
	// UserID is only stored for IM/DM's so we can then send a user a DM as a
	// response if needed
}

// init registers SlackAdapter to the victor chat framework.
func init() {
	chat.Register(AdapterName, func(r chat.Robot) chat.Adapter {
		config, configSet := r.AdapterConfig()
		if !configSet {
			log.Println("A configuration struct implementing the SlackConfig interface must be set.")
			os.Exit(1)
		}
		sConfig, ok := config.(Config)
		if !ok {
			log.Println("The bot's config must implement the SlackConfig interface.")
			os.Exit(1)
		}
		return &SlackAdapter{
			robot:            r,
			chReceiver:       make(chan slack.SlackEvent),
			token:            sConfig.Token(),
			channelInfo:      make(map[string]channelGroupInfo),
			directMessageID:  make(map[string]string),
			botRegex:         regexp.MustCompile(fmt.Sprintf(botRegexFormat, r.Name())),
			botCommandPrefix: fmt.Sprintf("@%s: ", r.Name()),
		}
	})
}

// Config provides the slack adapter with the necessary
// information to open a websocket connection with the slack Real time API.
type Config interface {
	Token() string
}

// Config implements the SlackRealtimeConfig interface to provide a slack
// adapter with the information it needs to authenticate with slack.
type configImpl struct {
	token string
}

// NewConfig returns a new slack configuration instance using the given token.
func NewConfig(token string) configImpl {
	return configImpl{token: token}
}

// Token returns the slack token.
func (c configImpl) Token() string {
	return c.token
}

// SlackAdapter holds all information needed by the adapter to send/receive messages.
type SlackAdapter struct {
	robot            chat.Robot
	token            string
	instance         *slack.Slack
	wsAPI            *slack.SlackWS
	chReceiver       chan slack.SlackEvent
	channelInfo      map[string]channelGroupInfo
	directMessageID  map[string]string
	botRegex         *regexp.Regexp
	botCommandPrefix string
}

// Run starts the adapter and begins to listen for new messages to send/receive.
// At the moment this will crash the program and print the error messages to a
// log if the connection fails.
func (adapter *SlackAdapter) Run() {
	adapter.instance = slack.New(adapter.token)
	adapter.instance.SetDebug(false)
	// TODO need to look up what these values actually mean...
	var err error
	adapter.wsAPI, err = adapter.instance.StartRTM("", "http://example.com")
	if err != nil {
		log.Println(err.Error())
		os.Exit(1)
	}
	adapter.initChannelMap()
	// sets up the monitoring code for sending/receiving messages from slack
	go adapter.wsAPI.HandleIncomingEvents(adapter.chReceiver)
	go adapter.wsAPI.Keepalive(20 * time.Second)
	adapter.monitorEvents()
}

func (adapter *SlackAdapter) initChannelMap() {
	channels, err := adapter.instance.GetChannels(true)
	if err != nil {
		log.Printf("Error getting channel list: %s", err.Error())
		return
	}
	groups, err := adapter.instance.GetGroups(true)
	if err != nil {
		log.Printf("Error getting group list: %s", err.Error())
		return
	}
	ims, err := adapter.instance.GetIMChannels()
	if err != nil {
		log.Printf("Error getting IM (DM) channel list: %s", err.Error())
		return
	}
	for _, channel := range channels {
		if !channel.IsMember {
			continue
		}
		// log.Printf("Loaded info for channel \"%s\"", channel.Name)
		adapter.channelInfo[channel.Id] = channelGroupInfo{
			ID:        channel.Id,
			Name:      channel.Name,
			IsChannel: true,
		}
	}
	for _, group := range groups {
		// log.Printf("Loaded info for group \"%s\"", group.Name)
		adapter.channelInfo[group.Id] = channelGroupInfo{
			ID:   group.Id,
			Name: group.Name,
		}
	}
	for _, im := range ims {
		// log.Printf("Loaded info for IM \"%s\"", im.Id)
		adapter.channelInfo[im.Id] = channelGroupInfo{
			ID:     im.Id,
			Name:   fmt.Sprintf("DM %s", im.Id),
			IsDM:   true,
			UserID: im.UserId,
		}
		adapter.directMessageID[im.UserId] = im.Id
	}
}

// Stop stops the adapter.
// TODO implement
func (adapter *SlackAdapter) Stop() {
}

func (adapter *SlackAdapter) getUser(userID string) (*slack.User, error) {
	// try to get the stored user info
	userStr, exists := adapter.getStoreKey(userID)
	// if it hasn't been stored then perform a slack API call to get it and
	// store it
	if !exists {
		user, err := adapter.instance.GetUserInfo(userID)
		if err != nil {
			log.Println(err.Error())
			return nil, err
		}
		// try to encode it as a json string for storage
		var userArr []byte
		if userArr, err = json.Marshal(user); err == nil {
			adapter.setStoreKey(userID, string(userArr))
		}
		return user, nil
	}
	var user slack.User
	// convert the json string to the user object
	err := json.Unmarshal([]byte(userStr), &user)
	if err != nil {
		log.Println(err.Error())
		return nil, err
	}
	return &user, nil

	// TODO handle an error on the unmarshalling of the stored json object?
}

func (adapter *SlackAdapter) handleMessage(event *slack.MessageEvent) {
	user, _ := adapter.getUser(event.UserId)
	channel, exists := adapter.channelInfo[event.ChannelId]
	if !exists {
		log.Printf("Unrecognized channel with ID %s", event.Id)
		channel = channelGroupInfo{
			Name: "Unrecognized",
			ID:   event.ChannelId,
		}
	}
	// TODO use error
	if user != nil {
		// ignore any messages that are sent by any bot
		if user.IsBot {
			return
		}
		messageText := adapter.unescapeMessage(event.Text)
		msg := slackMessage{
			user:            user,
			text:            messageText,
			channelID:       channel.ID,
			channelName:     channel.Name,
			isDirectMessage: channel.IsDM,
		}
		adapter.robot.Receive(&msg)
		log.Println(msg.Text())
	}
}

// Replace all instances of the bot's encoded name with it's actual name.
//
// TODO might want to update this to replace all matches of <@USER_ID> with
// the user's name.
func (adapter *SlackAdapter) unescapeMessage(msg string) string {
	userID := adapter.instance.GetInfo().User.Id
	return strings.Replace(msg, getEncodedUserID(userID), adapter.robot.Name(), -1)
}

// Returns the encoded string version of a user's slack ID.
func getEncodedUserID(userID string) string {
	return fmt.Sprintf("<@%s>", userID)
}

// monitorEvents handles incoming events and filters them to only worry about
// incoming messages.
func (adapter *SlackAdapter) monitorEvents() {
	for {
		msg := <-adapter.chReceiver
		switch msg.Data.(type) {
		case *slack.MessageEvent:
			go adapter.handleMessage(msg.Data.(*slack.MessageEvent))
		case *slack.ChannelJoinedEvent:
			go adapter.joinedChannel(msg.Data.(*slack.ChannelJoinedEvent).Channel, true)
		case *slack.GroupJoinedEvent:
			go adapter.joinedChannel(msg.Data.(*slack.GroupJoinedEvent).Channel, false)
		case *slack.IMCreatedEvent:
			// could also use im open? https://api.slack.com/events/im_created
			go adapter.joinedIM(msg.Data.(*slack.IMCreatedEvent))
		case *slack.ChannelLeftEvent:
			go adapter.leftChannel(msg.Data.(*slack.ChannelLeftEvent).ChannelId)
		case *slack.GroupLeftEvent:
			go adapter.leftChannel(msg.Data.(*slack.GroupLeftEvent).ChannelId)
		case *slack.IMCloseEvent:
			go adapter.leftIM(msg.Data.(*slack.IMCloseEvent))
		}
	}
}

func (adapter *SlackAdapter) joinedChannel(channel slack.Channel, isChannel bool) {
	// log.Printf("Loaded info for channel/group \"%s\"", channel.Name)
	adapter.channelInfo[channel.Id] = channelGroupInfo{
		Name:      channel.Name,
		ID:        channel.Id,
		IsChannel: isChannel,
	}
}

func (adapter *SlackAdapter) joinedIM(event *slack.IMCreatedEvent) {
	// log.Printf("Loaded info for IM \"%s\"", event.Channel.Id)
	adapter.channelInfo[event.Channel.Id] = channelGroupInfo{
		Name:   event.Channel.Name,
		ID:     event.Channel.Id,
		IsDM:   true,
		UserID: event.UserId,
	}
	adapter.directMessageID[event.UserId] = event.Channel.Id
}

func (adapter *SlackAdapter) leftIM(event *slack.IMCloseEvent) {
	adapter.leftChannel(event.ChannelId)
	delete(adapter.directMessageID, event.UserId)
}

func (adapter *SlackAdapter) leftChannel(channelID string) {
	// log.Printf("Forgetting channel/group with ID %s", channelID)
	delete(adapter.channelInfo, channelID)
}

// Send sends a message to the given slack channel.
func (adapter *SlackAdapter) Send(channelID, msg string) {
	msgObj := adapter.wsAPI.NewOutgoingMessage(msg, channelID)
	adapter.wsAPI.SendMessage(msgObj)
}

func (adapter *SlackAdapter) SendDirectMessage(userID, msg string) {
	channelID, err := adapter.getDirectMessageID(userID)
	if err != nil {
		log.Printf("Error getting direct message channel ID for user \"%s\": %s", userID, err.Error())
		return
	}
	adapter.Send(channelID, msg)
}

func (adapter *SlackAdapter) getDirectMessageID(userID string) (string, error) {
	// need to figure out if the first two bool return values are important
	// https://github.com/nlopes/slack/blob/master/dm.go#L58
	channel, exists := adapter.channelInfo[userID]
	if !exists {
		_, _, channelID, err := adapter.instance.OpenIMChannel(userID)
		return channelID, err
	}
	return channel.ID, nil
}

// getStoreKey is a helper method to access the robot's store.
func (adapter *SlackAdapter) getStoreKey(key string) (string, bool) {
	return adapter.robot.Store().Get(userInfoPrefix + key)
}

// setStoreKey is a helper method to access the robot's store.
func (adapter *SlackAdapter) setStoreKey(key, val string) {
	adapter.robot.Store().Set(userInfoPrefix+key, val)
}

// slackMessage is an internal struct implementing victor's message interface.
type slackMessage struct {
	user            *slack.User
	text            string
	channelID       string
	channelName     string
	isDirectMessage bool
}

func (m *slackMessage) IsDirectMessage() bool {
	return m.isDirectMessage
}

func (m *slackMessage) User() *slack.User {
	return m.user
}

func (m *slackMessage) UserID() string {
	return m.user.Id
}

func (m *slackMessage) UserName() string {
	return m.user.Name
}

func (m *slackMessage) EmailAddress() string {
	return m.user.Profile.Email
}

func (m *slackMessage) ChannelID() string {
	return m.channelID
}

func (m *slackMessage) ChannelName() string {
	return m.channelName
}

func (m *slackMessage) Text() string {
	return m.text
}

func (m *slackMessage) SetText(newText string) {
	m.text = newText
}
