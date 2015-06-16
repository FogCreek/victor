package victor

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/brettbuddin/victor/pkg/chat"
	"github.com/mattn/go-shellwords"
)

// Printf style format for a bot's name regular expression
const botNameRegexFormat = "(?i)^(?:@)?%s\\s*[:,]?\\s*\\b"

// Pre-compiled regular expression to match a word that starts a string
var wordRegex = regexp.MustCompile("^\\S+")

// HandlerPair provides an interface for a handler as well as the regular
// expression which a message should match in order to pass control onto the
// handler
type HandlerPair interface {
	Exp() *regexp.Regexp
	Handler() HandlerFunc
}

type handlerPair struct {
	exp    *regexp.Regexp
	handle HandlerFunc
}

func (pair *handlerPair) Exp() *regexp.Regexp {
	return pair.exp
}

func (pair *handlerPair) Handler() HandlerFunc {
	return pair.handle
}

type dispatch struct {
	robot          Robot
	handlers       []HandlerPair
	defaultHandler HandlerFunc
	commands       map[string]HandlerFunc
	patterns       []HandlerPair
	botNameRegex   *regexp.Regexp
}

func newDispatch(bot Robot) *dispatch {
	return &dispatch{
		robot:          bot,
		defaultHandler: nil,
		commands:       make(map[string]HandlerFunc),
		patterns:       make([]HandlerPair, 0, 10),
		botNameRegex:   regexp.MustCompile(fmt.Sprintf(botNameRegexFormat, bot.Name())),
	}
}

// HandleCommand adds a given string/handler pair as a new command for the bot.
// This will call the handler function if a string insensitive match succeeds
// on the command name of a message that is considered a potential command
// (either sent @ the bot's name or in a direct message).
func (d *dispatch) HandleCommand(name string, handler HandlerFunc) {
	lowerName := strings.ToLower(name)
	if _, exists := d.commands[lowerName]; exists {
		log.Printf("\"%s\" has been set more than once.", lowerName)
	}
	d.commands[lowerName] = handler
}

// SetDefaultHandler sets a function as the default handler which is called
// when a potential command message (either sent @ the bot's name or in a
// direct message) does not match any of the other set commands.
func (d *dispatch) SetDefaultHandler(handler HandlerFunc) {
	if d.defaultHandler != nil {
		log.Println("Default handler has been set more than once.")
	}
	d.defaultHandler = handler
}

// HandlePattern adds a given pattern to the bot's list of regexp expressions.
// This pattern will be checked on every message that is not considered a
// potential command (NOT sent @ the bot and NOT in a direct message).
// If multiple patterns are added then they will be evaluated in the order of
// insertion.
func (d *dispatch) HandlePattern(pattern string, handler HandlerFunc) {
	d.patterns = append(d.patterns, &handlerPair{
		exp:    regexp.MustCompile(pattern),
		handle: handler,
	})
}

// ProcessMessage finds a match for a message and runs its Handler.
// If the message is considered a potential command (either sent @ the bot's
// name or in a direct message) then the next word after the bot's name is
// compared to all registered handlers. If one matches then that handler
// function is called with the remaining text seperated by whitespace
// (restpecting quotes). If none match then the default handler is called
// (with an empty fields array).
//
// If the message is not a potential command then it is checked against all
// registered patterns (with an empty fields array upon a match).
func (d *dispatch) ProcessMessage(m chat.Message) {
	messageText := m.Text()
	nameMatch := d.botNameRegex.FindString(messageText)
	if len(nameMatch) > 0 || m.IsDirectMessage() {
		// slices are cheap (reference original) so if no match then it's ok
		messageText = messageText[len(nameMatch):]
		if !d.matchCommands(m, messageText) {
			d.callDefault(m)
		}
	} else if len(nameMatch) == 0 {
		d.matchPatterns(m)
	}
}

// callDefault invokes the default message handler if one is set.
// If one is not set then it logs the unhandled occurrance but otherwise does
// not fail.
func (d *dispatch) callDefault(m chat.Message) {
	if d.defaultHandler != nil {
		d.defaultHandler.Handle(&state{
			robot:   d.robot,
			message: m,
		})
	} else {
		log.Println("Default handler invoked but none is set.")
	}
}

// matchCommands attempts to match a given message with the map of registered
// commands. It performs case-insensitive matching and will return true upon
// the first match. It expects the second parameter to be the message's text
// with the bot's name and any follwing up until the next word whitespace
// removed.
func (d *dispatch) matchCommands(m chat.Message, messageText string) bool {
	commandName := strings.ToLower(wordRegex.FindString(messageText))
	if len(commandName) == 0 {
		return false
	}
	command, defined := d.commands[commandName]
	if !defined {
		return false
	}
	remainingText := messageText
	remainingText = strings.TrimSpace(remainingText[len(commandName):])
	fields, err := shellwords.Parse(remainingText)
	if err != nil {
		log.Println(err.Error())
	}
	command.Handle(&state{
		robot:   d.robot,
		message: m,
		fields:  fields,
	})
	return true
}

// matchPatterns iterates through the array of registered regular expressions
// (patterns) in the order of insertion and checks if they match any part of
// the given message's text. If they do then they are invoked with an empty
// fields array.
func (d *dispatch) matchPatterns(m chat.Message) {
	for _, pair := range d.patterns {
		if pair.Exp().MatchString(m.Text()) {
			pair.Handler().Handle(&state{
				robot:   d.robot,
				message: m,
			})
			return
		}
	}
}
