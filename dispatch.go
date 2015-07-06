package victor

import (
	"bytes"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/FogCreek/victor/pkg/chat"
)

// Printf style format for a bot's name regular expression
const botNameRegexFormat = "(?i)^(?:@)?%s\\s*[:,]?\\s*"

// Pre-compiled regular expression to match a word that starts a string
var wordRegex = regexp.MustCompile("^(\\S+)")

const helpCommandName = "help"

// HandlerDocPair provides a common interface for command handlers to be added
// to a victor Robot along with their name, description, and usage. This allows
// for a "help" handler to be easily written.
type HandlerDocPair interface {
	Handler() HandlerFunc
	Name() string
	Description() string
	Usage() []string
	IsHidden() bool
}

// HandlerDoc provides a base implementation of the HandlerDocPair interface.
type HandlerDoc struct {
	CmdHandler     HandlerFunc
	CmdName        string
	CmdDescription string
	CmdUsage       []string
	CmdIsHidden    bool
}

// IsHidden returns true if this command should be hidden from the help list of
// commands. It will still be "visible" if accessed with help by name.
func (d *HandlerDoc) IsHidden() bool {
	return d.CmdIsHidden
}

// Handler returns the HandlerFunc.
func (d *HandlerDoc) Handler() HandlerFunc {
	return d.CmdHandler
}

// Name returns the handler's set command name. This is not guarenteed to be
// normalized (all lower case).
func (d *HandlerDoc) Name() string {
	return d.CmdName
}

// Description returns the command's set description.
func (d *HandlerDoc) Description() string {
	return d.CmdDescription
}

// Usage returns an array of acceptable usages for this command.
// The usages should not have the command's name in them in order to work
// with the default help handler.
func (d *HandlerDoc) Usage() []string {
	return d.CmdUsage
}

// Set up base default help handler. Before use a copy msut be made and the
// CmdHandler property must be set.
var defaultHelpHandlerDoc = HandlerDoc{
	CmdName:        helpCommandName,
	CmdDescription: "View list of commands and their usage.",
	CmdUsage:       []string{"", "`command name`"},
}

// HandlerRegExpPair provides an interface for a handler as well as the regular
// expression which a message should match in order to pass control onto the
// handler
type HandlerRegExpPair interface {
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
	defaultHandler HandlerFunc
	commands       map[string]HandlerDocPair
	commandNames   []string
	patterns       []HandlerRegExpPair
	botNameRegex   *regexp.Regexp
}

func newDispatch(bot Robot) *dispatch {
	return &dispatch{
		robot:          bot,
		defaultHandler: nil,
		commands:       make(map[string]HandlerDocPair),
		commandNames:   make([]string, 0, 10),
		patterns:       make([]HandlerRegExpPair, 0, 10),
		botNameRegex:   regexp.MustCompile(fmt.Sprintf(botNameRegexFormat, bot.Name())),
	}
}

func (d *dispatch) Commands() map[string]HandlerDocPair {
	cmdCopy := make(map[string]HandlerDocPair)
	for key, value := range d.commands {
		cmdCopy[key] = value
	}
	return cmdCopy
}

func (d *dispatch) EnableHelpCommand() {
	if _, exists := d.commands[helpCommandName]; exists {
		log.Println("Enabling built in help command and overriding set help command.")
	}
	// make a copy of it and use a closure to provide access to the current
	// dispatch
	helpHandler := defaultHelpHandlerDoc
	helpHandler.CmdHandler = func(s State) {
		defaultHelpHandler(s, d)
	}
	d.HandleCommand(&helpHandler)
}

// HandleCommand adds a given string/handler pair as a new command for the bot.
// This will call the handler function if a string insensitive match succeeds
// on the command name of a message that is considered a potential command
// (either sent @ the bot's name or in a direct message).
func (d *dispatch) HandleCommand(cmd HandlerDocPair) {
	lowerName := strings.ToLower(cmd.Name())
	newCommand := true
	if _, exists := d.commands[lowerName]; exists {
		log.Printf("\"%s\" has been set more than once.", lowerName)
		newCommand = false
	}
	d.commands[lowerName] = cmd
	if newCommand {
		d.commandNames = append(d.commandNames, cmd.Name())
		sort.Strings(d.commandNames)
	}
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
// This is equivalent to calling HandleRegexp with a pre-compiled regular
// expression. This uses regexp.MustCompile so it panics if given an
// invalid regular expression.
func (d *dispatch) HandlePattern(pattern string, handler HandlerFunc) {
	d.HandleRegexp(regexp.MustCompile(pattern), handler)
}

// HandleRegexp adds a given regular expression to the bot's list of regexp
// expressions. This expression will be checked on every message that is not
// considered a potential command (NOT sent @ the bot and NOT in a
// direct message). If multiple expressions are added then they will be
// evaluated in the order of insertion.
func (d *dispatch) HandleRegexp(exp *regexp.Regexp, handler HandlerFunc) {
	d.patterns = append(d.patterns, &handlerPair{
		exp:    exp,
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
			d.callDefault(m, messageText)
		}
	} else if len(nameMatch) == 0 {
		d.matchPatterns(m)
	}
}

func getFields(messageText, commandName string) []string {
	remainingText := strings.TrimSpace(messageText[len(commandName):])
	return parseFields(remainingText)
}

// callDefault invokes the default message handler if one is set.
// If one is not set then it logs the unhandled occurrance but otherwise does
// not fail.
func (d *dispatch) callDefault(m chat.Message, messageText string) {
	fields := getFields(messageText, "")
	if d.defaultHandler != nil {
		d.defaultHandler.Handle(&state{
			robot:   d.robot,
			message: m,
			fields:  fields,
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
	fields := getFields(messageText, commandName)
	command.Handler().Handle(&state{
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

func defaultHelpHandler(s State, d *dispatch) {
	if len(s.Fields()) == 0 {
		showAllCommands(s, d)
	} else {
		showCommandHelp(s, d)
	}
}

func showAllCommands(s State, d *dispatch) {
	if len(d.commandNames) == 0 {
		s.Chat().Send(s.Message().ChannelID(), "No commands have been set!")
		return
	}
	var buf bytes.Buffer
	buf.WriteString("Available commands:\n")
	buf.WriteString(">>>")
	for _, name := range d.commandNames {
		docPair, ok := d.commands[name]
		if !ok || docPair.IsHidden() {
			continue
		}
		buf.WriteString("*")
		buf.WriteString(docPair.Name())
		buf.WriteString("* - _")
		buf.WriteString(docPair.Description())
		buf.WriteString("_\n")
	}
	buf.WriteString("\nFor help with a command, type `help [command name]`.")
	s.Chat().Send(s.Message().ChannelID(), buf.String())
}

func showCommandHelp(s State, d *dispatch) {
	if len(s.Fields()) == 0 {
		return
	}
	cmdName := strings.ToLower(s.Fields()[0])
	docPair, exists := d.commands[cmdName]
	if !exists {
		textFmt := "Unrecognized command _%s_.  Type *`help`* to view a list of all available commands."
		s.Chat().Send(s.Message().ChannelID(), fmt.Sprintf(textFmt, cmdName))
		return
	}
	var buf bytes.Buffer
	buf.WriteString("*")
	buf.WriteString(docPair.Name())
	buf.WriteString("* - _")
	buf.WriteString(docPair.Description())
	buf.WriteString("_\n\n")
	buf.WriteString(">>>")
	for _, use := range docPair.Usage() {
		buf.WriteString(docPair.Name())
		buf.WriteString(" ")
		buf.WriteString(use)
		buf.WriteString("\n")
	}
	s.Chat().Send(s.Message().ChannelID(), buf.String())
}

var quoteCharacters = &unicode.RangeTable{
	R16: []unicode.Range16{
		unicode.Range16{Lo: '"', Hi: '"', Stride: 1},
	},
}

func parseFields(input string) []string {
	fields := make([]string, 0, 10)
	skipSpaces := true
	fieldStart := -1

	for i, r := range input {
		if unicode.In(r, quoteCharacters) {
			if fieldStart == -1 {
				// start field
				fieldStart = i + 1
				skipSpaces = false
			} else {
				// end field
				fields = append(fields, input[fieldStart:i])
				fieldStart = -1
				skipSpaces = true
			}
		} else if unicode.IsSpace(r) && skipSpaces {
			// end field if not in a quoted field
			if fieldStart != -1 {
				fields = append(fields, input[fieldStart:i])
				fieldStart = -1
			}
		} else if fieldStart == -1 {
			// start field
			fieldStart = i
		}
	}
	if fieldStart != -1 {
		// end last field if it hasn't yet
		fields = append(fields, input[fieldStart:])
	}
	return fields
}
