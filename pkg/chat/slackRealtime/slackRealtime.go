package slackRealtime

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/FogCreek/slack"
	"github.com/FogCreek/victor/pkg/chat"
	"github.com/FogCreek/victor/pkg/events"
	"github.com/FogCreek/victor/pkg/events/definedEvents"
)

const (
	// TokenLength is the expected length of a Slack API auth token.
	TokenLength = 40

	// AdapterName is the Slack Websocket's registered adapter name for the
	// victor framework.
	AdapterName = "slackRealtime"

	// archiveURLFormat defines a printf-style format string for building
	// archive links centered around a message using the slack instance's
	// team name, the channel name, and the message's timestamp.
	archiveURLFormat = "https://%s.slack.com/archives/%s/p%s"
)

var (
	// Match "<@Userid>" and "<@UserID|fullname>"
	userIDRegexp = regexp.MustCompile(`^<@(U[[:alnum:]]+)(?:(?:|\S+)?>)`)

	// Match "<#ChannelID>" and "<#ChannelID|name>"
	channelIDRegexp = regexp.MustCompile(`^<#(C[[:alnum:]]+)(?:(?:|\S+)?>)`)

	// Should match all formatted slack inputs and have a capturing group of
	// the desired value from the formatted group.
	formattingRegexp = regexp.MustCompile(`<(?:mailto\:)?([^\|>]+)\|?[^>]*>`)

	// If a message part starts with any of these prefixes (case sensitive)
	// then it should not be unformatted by "unescapeMessage".
	unformattedPrefixes = []string{"@U", "#C", "!"}
)

// channelGroupInfo is used instead of the slack library's Channel struct since we
// are trying to consider channels and groups to be roughly the same while it
// considers them seperate and provides no way to consolidate them on its own.
//
// This also allows us to throw our information that we don't care about (members, etc.).
type channelGroupInfo struct {
	Name      string
	ID        string
	IsDM      bool
	IsChannel bool
	UserID    string
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
	robot            chat.Robot
	token            string
	instance         *slack.Client
	rtm              *slack.RTM
	chReceiver       chan slack.SlackEvent
	channelInfo      map[string]channelGroupInfo
	directMessageID  map[string]string
	userInfo         map[string]slack.User
	domain           string
	botID            string
	formattedSlackID string
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
	userID := normalizeID(userIDStr, userIDRegexp)
	userObj, err := adapter.getUserFromSlack(userID)
	if err != nil {
		log.Println("Error getting user:", err.Error())
		return nil
	}
	return &chat.BaseUser{
		UserID:    userObj.Id,
		UserName:  userObj.Name,
		UserEmail: userObj.Profile.Email,
		UserIsBot: userObj.IsBot,
	}
}

func (adapter *SlackAdapter) GetChannel(channelIDStr string) chat.Channel {
	if !adapter.IsPotentialChannel(channelIDStr) {
		log.Printf("%s is not a potential channel", channelIDStr)
		return nil
	}
	channelID := normalizeID(channelIDStr, channelIDRegexp)
	channelObj := adapter.getChannelFromSlack(channelID)
	if channelObj.Name == "Unrecognized" {
		return nil
	}
	return &chat.BaseChannel{
		ChannelID:   channelObj.ID,
		ChannelName: channelObj.Name,
	}
}

// normalizeID returns an ID without the extra formatting that slack might add.
//
// This returns the first captured field of the first submatch using the given
// precompiled regexp. If no matches are found or no captured groups are
// defined then this returns the input text unchanged.
func normalizeID(id string, exp *regexp.Regexp) string {
	idArr := exp.FindAllStringSubmatch(id, 1)
	if len(idArr) == 0 || len(idArr[0]) < 2 {
		return id
	}
	return idArr[0][1]
}

// GetAllUsers returns a slice of all user objects that are known to the
// chatbot. This does not perform a slack API call as all users should be
// stored locally and any new users will be added upon a team join event.
func (adapter *SlackAdapter) GetAllUsers() []chat.User {
	var users []chat.User
	for _, u := range adapter.userInfo {
		users = append(users, &chat.BaseUser{
			UserID:    u.Id,
			UserName:  u.Name,
			UserEmail: u.Profile.Email,
			UserIsBot: u.IsBot,
		})
	}
	return users
}

// GetPublicChannels returns a slice of all channels that are known to the
// chatbot.
func (adapter *SlackAdapter) GetPublicChannels() []chat.Channel {
	var channels []chat.Channel
	for _, c := range adapter.channelInfo {
		if c.IsChannel {
			channels = append(channels, &chat.BaseChannel{
				ChannelID:   c.ID,
				ChannelName: c.Name,
			})
		}
	}
	return channels
}

// IsPotentialUser checks if a given string is potentially referring to a slack
// user. Strings given to this function should be trimmed of leading whitespace
// as it does not account for that (it is meant to be used with the fields
// method on the frameworks calls to handlers which are trimmed).
func (adapter *SlackAdapter) IsPotentialUser(userString string) bool {
	return userIDRegexp.MatchString(userString)
}

// IsPotentialChannel checks if a given string is potentially referring to a
// slack channel. Strings given to this function should be trimmed of leading
// whitespace as it does not account for that (it is meant to be used with the
// fields method on the frameworks calls to handlers which are trimmed).
func (adapter *SlackAdapter) IsPotentialChannel(channelString string) bool {
	return channelIDRegexp.MatchString(channelString)
}

// Run starts the adapter and begins to listen for new messages to send/receive.
// At the moment this will crash the program and print the error messages to a
// log if the connection fails.
func (adapter *SlackAdapter) Run() {
	adapter.instance = slack.New(adapter.token)
	adapter.instance.SetDebug(false)
	adapter.rtm = adapter.instance.NewRTM()
	go adapter.monitorEvents()
	go adapter.rtm.ManageConnection()
}

func (adapter *SlackAdapter) initAdapterInfo(info *slack.Info) {
	adapter.formattedSlackID = fmt.Sprintf("<@%s>", info.User.Id)
	adapter.botID = info.User.Id
	adapter.domain = info.Team.Domain
	for _, channel := range info.Channels {
		if !channel.IsMember {
			continue
		}
		adapter.channelInfo[channel.Id] = channelGroupInfo{
			ID:        channel.Id,
			Name:      channel.Name,
			IsChannel: true,
			IsDM:      false,
		}
	}
	for _, group := range info.Groups {
		adapter.channelInfo[group.Id] = channelGroupInfo{
			ID:        group.Id,
			Name:      group.Name,
			IsChannel: false,
			IsDM:      false,
		}
	}
	for _, im := range info.IMs {
		adapter.channelInfo[im.Id] = channelGroupInfo{
			ID:        im.Id,
			Name:      fmt.Sprintf("DM %s", im.Id),
			IsChannel: false,
			IsDM:      true,
			UserID:    im.UserId,
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
	adapter.rtm.Disconnect()
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

func (adapter *SlackAdapter) getChannelFromSlack(channelID string) channelGroupInfo {
	channel, exists := adapter.channelInfo[channelID]
	if exists {
		return channel
	}
	channelObj, err := adapter.rtm.GetChannelInfo(channelID)
	if err != nil {
		log.Printf("Unrecognized channel with ID %s", channelID)
		return channelGroupInfo{
			Name: "Unrecognized",
			ID:   channelID,
		}
	}
	info := channelGroupInfo{
		ID:        channelObj.Id,
		Name:      channelObj.Name,
		IsChannel: true,
	}
	adapter.channelInfo[channelObj.Id] = info
	return info
}

func (adapter *SlackAdapter) handleMessage(event *slack.MessageEvent) {
	if len(event.SubType) > 0 {
		return
	}
	user, _ := adapter.getUserFromSlack(event.UserId)
	channel := adapter.getChannelFromSlack(event.ChannelId)
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
			MsgChannel: &chat.BaseChannel{
				ChannelID:   channel.ID,
				ChannelName: channel.Name,
			},
			MsgText:        messageText,
			MsgIsDirect:    channel.IsDM,
			MsgTimestamp:   strings.SplitN(event.Timestamp, ".", 2)[0],
			MsgArchiveLink: archiveLink,
		}
		adapter.robot.Receive(&msg)
	}
}

func (adapter *SlackAdapter) getArchiveLink(channelName, timestamp string) string {
	return fmt.Sprintf(archiveURLFormat, adapter.domain, channelName, strings.Replace(timestamp, ".", "", 1))
}

// Fix formatting on incoming slack messages.
//
// This will also check if the message starts with the bot's user id. If it
// does then it replaces it with the text version of the bot's name
// (ex: "@victor") so the victor dispatch can recognize it as being directed
// at the bot.
func (adapter *SlackAdapter) unescapeMessage(msg string) string {
	// special case for starting with the bot's name
	// could replace all instances of bot's name but we only care about the
	// first one and all subsequent occurrences will be in the same format
	// as other user names.
	if strings.HasPrefix(msg, adapter.formattedSlackID) {
		msg = "@" + adapter.robot.Name() + msg[len(adapter.formattedSlackID):]
	}
	// find all formatted parts of the message
	matches := formattingRegexp.FindAllStringSubmatch(msg, -1)
	for _, match := range matches {
		if shouldUnformat(match[1]) {
			// replace the full formatted string part with the captured value
			// from the "formattingRegexp" regex
			msg = strings.Replace(msg, match[0], match[1], 1)
		}
	}
	return msg
}

// shouldUnformat checks if a given formatted string from slack should be
// unformatted (remove brackets and optional pipe with name). This uses the
// "unformattedPrefixes" array and checks if the given string starts with one
// of those defined prefixes. If it does, then it should not be unformatted and
// this will return false. Otherwise this will return true but not perform the
// unformatting.
func shouldUnformat(part string) bool {
	for _, s := range unformattedPrefixes {
		if strings.HasPrefix(part, s) {
			return false
		}
	}
	return true
}

// monitorEvents handles incoming events and filters them to only worry about
// incoming messages.
func (adapter *SlackAdapter) monitorEvents() {
	errorChannel := adapter.robot.ChatErrors()
	eventChannel := adapter.robot.ChatEvents()
	for {
		event := <-adapter.rtm.IncomingEvents
		switch e := event.Data.(type) {
		case *slack.InvalidAuthEvent:
			errorChannel <- &definedEvents.InvalidAuth{}
		case *slack.ConnectingEvent:
			go func() {
				eventChannel <- &definedEvents.ConnectingEvent{}
			}()
		case *slack.ConnectedEvent:
			go func() {
				eventChannel <- &definedEvents.ConnectedEvent{}
			}()
			go adapter.initAdapterInfo(e.Info)
		case *slack.SlackWSError:
			errorChannel <- &events.BaseError{
				ErrorObj: e,
			}
		case *slack.DisconnectedEvent:
			errorChannel <- &definedEvents.Disconnect{
				Intentional: e.Intentional,
			}
		case *slack.MessageEvent:
			go adapter.handleMessage(e)
		case *slack.ChannelJoinedEvent:
			go adapter.joinedChannel(e.Channel, true)
		case *slack.GroupJoinedEvent:
			go adapter.joinedChannel(e.Channel, false)
		case *slack.IMCreatedEvent:
			go adapter.joinedIM(e)
		case *slack.ChannelLeftEvent:
			go adapter.leftChannel(e.ChannelId, true)
		case *slack.GroupLeftEvent:
			go adapter.leftChannel(e.ChannelId, false)
		case *slack.IMCloseEvent:
			go adapter.leftIM(e)
		case *slack.TeamDomainChangeEvent:
			go adapter.domainChanged(e)
		case *slack.UserChangeEvent:
			go adapter.userChanged(e.User)
		case *slack.TeamJoinEvent:
			go adapter.userChanged(*e.User)
			eventChannel <- &definedEvents.UserEvent{
				User: &chat.BaseUser{
					UserID:    e.User.Id,
					UserName:  e.User.Name,
					UserEmail: e.User.Profile.Email,
					UserIsBot: e.User.IsBot,
				},
				WasRemoved: false,
			}
		case *slack.UnmarshallingErrorEvent:
			errorChannel <- &events.BaseError{
				ErrorObj: e.ErrorObj,
			}
		case *slack.OutgoingErrorEvent:
			errorChannel <- &events.BaseError{
				ErrorObj: e.ErrorObj,
			}
		case *slack.MessageTooLongEvent:
			errorChannel <- &definedEvents.MessageTooLong{
				MaxLength: e.MaxLength,
				Text:      e.Message.Text,
				ChannelID: e.Message.ChannelId,
			}
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
	adapter.channelInfo[channel.Id] = channelGroupInfo{
		Name:      channel.Name,
		ID:        channel.Id,
		IsChannel: isChannel,
	}
	if isChannel {
		adapter.robot.ChatEvents() <- &definedEvents.ChannelEvent{
			Channel: &chat.BaseChannel{
				ChannelName: channel.Name,
				ChannelID:   channel.Id,
			},
			WasRemoved: false,
		}
	}
}

func (adapter *SlackAdapter) joinedIM(event *slack.IMCreatedEvent) {
	adapter.channelInfo[event.Channel.Id] = channelGroupInfo{
		Name:   fmt.Sprintf("DM %s", event.Channel.Id),
		ID:     event.Channel.Id,
		IsDM:   true,
		UserID: event.UserId,
	}
	adapter.directMessageID[event.UserId] = event.Channel.Id
}

func (adapter *SlackAdapter) leftIM(event *slack.IMCloseEvent) {
	adapter.leftChannel(event.ChannelId, false)
	delete(adapter.directMessageID, event.UserId)
}

func (adapter *SlackAdapter) leftChannel(channelID string, isChannel bool) {
	channelName := adapter.channelInfo[channelID].Name
	delete(adapter.channelInfo, channelID)
	if isChannel {
		adapter.robot.ChatEvents() <- &definedEvents.ChannelEvent{
			Channel: &chat.BaseChannel{
				ChannelName: channelName,
				ChannelID:   channelID,
			},
			WasRemoved: true,
		}
	}
}

// Send sends a message to the given slack channel.
func (adapter *SlackAdapter) Send(channelID, msg string) {
	msgObj := adapter.rtm.NewOutgoingMessage(msg, channelID)
	adapter.rtm.SendMessage(msgObj)
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

func (adapter *SlackAdapter) SendTyping(channelID string) {
	adapter.rtm.SendMessage(&slack.OutgoingMessage{Type: "typing", ChannelId: channelID})
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
