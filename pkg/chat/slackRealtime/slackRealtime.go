package slackRealtime

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/FogCreek/victor/pkg/chat"
	"github.com/nlopes/slack"
)

// The Slack Websocket's registered adapter name for the victor framework.
const AdapterName = "slackRealtime"

// Prefix for the user's ID which is used when reading/writing from the bot's store
const userInfoPrefix = AdapterName + "."

const userIDRegexpString = "\\b<?@?(U[[:alnum:]]+)(?:(?:|\\S+)?>?)"

// Match "<@Userid>" and "<@UserID|fullname>"
var userIDRegexp = regexp.MustCompile(userIDRegexpString)

// Match "johndoe", "@johndoe",
// not needed?
// var userIDAndNameRegexp = regexp.MustCompile("\\A@?(\\w+)|" + userIDRegexpString)

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
			robot:           r,
			chReceiver:      make(chan slack.SlackEvent),
			token:           sConfig.Token(),
			channelInfo:     make(map[string]channelGroupInfo),
			directMessageID: make(map[string]string),
			userInfo:        make(map[string]slack.User),
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
	robot           chat.Robot
	token           string
	instance        *slack.Slack
	wsAPI           *slack.SlackWS
	chReceiver      chan slack.SlackEvent
	channelInfo     map[string]channelGroupInfo
	directMessageID map[string]string
	userInfo        map[string]slack.User
	domain          string
}

// GetUser will parse the given user ID string and then return the user's
// information as provided by the slack API. This will first try to get the
// user's information from a local cache and then will perform a slack API
// call if the user's information is not cached. Returns nil if the user does
// not exist or if an error occurrs during the slack API call.
func (adapter *SlackAdapter) GetUser(userIDStr string) chat.User {
	if !adapter.IsPotentialUser(userIDStr) {
		log.Printf("%s is not a potential user", userIDStr)
		return nil
	}
	userID := adapter.NormalizeUserID(userIDStr)
	userObj, err := adapter.getUserFromSlack(userID)
	if err != nil {
		log.Println("Error getting user: " + err.Error())
		return nil
	}
	return &chat.BaseUser{
		UserID:    userObj.Id,
		UserName:  userObj.Name,
		UserEmail: userObj.Profile.Email,
		UserIsBot: userObj.IsBot,
	}
}

// IsPotentialUser checks if a given string is potentially referring to a slack
// user. Strings given to this function should be trimmed of leading whitespace
// as it does not account for that (it is meant to be used with the fields
// method on the frameworks calls to handlers which are trimmed).
func (adapter *SlackAdapter) IsPotentialUser(userString string) bool {
	return userIDRegexp.MatchString(userString)
}

// normalizeUserID returns a user's ID without the extra formatting that slack
// might add. This will return "U01234567" for inputs: "U01234567",
// "@U01234567", "<@U01234567>", and "<@U01234567|name>"
func (adapter *SlackAdapter) NormalizeUserID(userID string) string {
	userIDArr := userIDRegexp.FindAllStringSubmatch(userID, 1)
	if len(userIDArr) == 0 {
		return userID
	}
	return userIDArr[0][1]
}

// Run starts the adapter and begins to listen for new messages to send/receive.
// At the moment this will crash the program and print the error messages to a
// log if the connection fails.
func (adapter *SlackAdapter) Run() error {
	adapter.instance = slack.New(adapter.token)
	adapter.instance.SetDebug(false)
	// TODO need to look up what these values actually mean...
	var err error
	adapter.wsAPI, err = adapter.instance.StartRTM("", "http://example.com")
	// TODO remove fatal crash or recover from it elsewhere
	if err != nil {
		return err
	}
	adapter.initAdapterInfo()
	// sets up the monitoring code for sending/receiving messages from slack
	go adapter.wsAPI.HandleIncomingEvents(adapter.chReceiver)
	go adapter.wsAPI.Keepalive(20 * time.Second)
	go adapter.monitorEvents()
	return nil
}

func (adapter *SlackAdapter) initAdapterInfo() {
	info := adapter.instance.GetInfo()
	adapter.domain = info.Team.Domain
	for _, channel := range info.Channels {
		if !channel.IsMember {
			continue
		}
		adapter.channelInfo[channel.Id] = channelGroupInfo{
			ID:        channel.Id,
			Name:      channel.Name,
			IsChannel: true,
		}
	}
	for _, group := range info.Groups {
		adapter.channelInfo[group.Id] = channelGroupInfo{
			ID:   group.Id,
			Name: group.Name,
		}
	}
	for _, im := range info.IMs {
		adapter.channelInfo[im.Id] = channelGroupInfo{
			ID:     im.Id,
			Name:   fmt.Sprintf("DM %s", im.Id),
			IsDM:   true,
			UserID: im.UserId,
		}
		adapter.directMessageID[im.UserId] = im.Id
	}
	for _, user := range info.Users {
		adapter.userInfo[user.Id] = user
	}
}

// Stop stops the adapter.
// TODO implement
func (adapter *SlackAdapter) Stop() {
}

// ID returns a unique ID for this adapter. At the moment this just returns
// the slack instance token but could be modified to return a uuid using a
// package such as https://godoc.org/code.google.com/p/go-uuid/uuid
func (adapter *SlackAdapter) ID() string {
	return adapter.token
}

func (adapter *SlackAdapter) getUserFromSlack(userID string) (*slack.User, error) {
	// try to get the stored user info
	user, exists := adapter.userInfo[userID]
	// if it hasn't been stored then perform a slack API call to get it and
	// store it
	if !exists {
		user, err := adapter.instance.GetUserInfo(userID)
		if err != nil {
			log.Println(err.Error())
			return nil, err
		}
		// try to encode it as a json string for storage
		adapter.userInfo[user.Id] = *user
		return user, nil
	}

	return &user, nil
}

func (adapter *SlackAdapter) handleMessage(event *slack.MessageEvent) {
	if len(event.SubType) > 0 {
		return
	}
	user, _ := adapter.getUserFromSlack(event.UserId)
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
		var archiveLink string
		if !channel.IsDM {
			archiveLink = adapter.getArchiveLink(channel.Name, event.Timestamp)
		} else {
			archiveLink = "No archive link for Direct Messages"
		}
		msg := chat.BaseMessage{
			MsgUser: &chat.BaseUser{
				UserID:    user.Id,
				UserName:  user.Name,
				UserEmail: user.Profile.Email,
			},
			MsgText:        messageText,
			MsgChannelID:   channel.ID,
			MsgChannelName: channel.Name,
			MsgIsDirect:    channel.IsDM,
			MsgTimestamp:   strings.SplitN(event.Timestamp, ".", 2)[0],
			MsgArchiveLink: archiveLink,
		}
		adapter.robot.Receive(&msg)
	}
}

const archiveURLFormat = "http://%s.slack.com/archives/%s/p%s"

func (adapter *SlackAdapter) getArchiveLink(channelName, timestamp string) string {
	return fmt.Sprintf(archiveURLFormat, adapter.domain, channelName, strings.Replace(timestamp, ".", "", 1))
}

// Replace all instances of the bot's encoded name with it's actual name.
//
// TODO might want to handle unescaping emails and urls here
func (adapter *SlackAdapter) unescapeMessage(msg string) string {
	userID := getEncodedUserID(adapter.instance.GetInfo().User.Id)
	if strings.HasPrefix(msg, userID) {
		return strings.Replace(msg, userID, adapter.robot.Name(), 1)
	}
	return msg
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
		case *slack.TeamDomainChangeEvent:
			go adapter.domainChanged(msg.Data.(*slack.TeamDomainChangeEvent))
		case *slack.UserChangeEvent:
			go adapter.userChanged(msg.Data.(*slack.UserChangeEvent).User)
		case *slack.TeamJoinEvent:
			// Need to dereference the user object due to inconsistencies in
			// the go slack api.
			go adapter.userChanged(*(msg.Data.(*slack.TeamJoinEvent).User))
		}
	}
}

func (adapter *SlackAdapter) userChanged(user slack.User) {
	if user.IsBot {
		return
	}
	adapter.userInfo[user.Id] = user
}

func (adapter *SlackAdapter) domainChanged(event *slack.TeamDomainChangeEvent) {
	adapter.domain = event.Domain
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

// SendDirectMessage sends the given message to the given user in a direct
// (private) message.
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
