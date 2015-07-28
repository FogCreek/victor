package victor

import (
	"bytes"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/FogCreek/victor/pkg/chat"
)

// Printf style format for a bot's name regular expression
const botNameRegexFormat = `(?i)^@%s\s*[:,]?\s*`

// Name of default "help" command that is added on a call to
// *dispatch.EnableHelpCommand().
const helpCommandName = "help"

// HandlerDocPair provides a common interface for command handlers to be added
// to a victor Robot along with their name, description, and usage. This allows
// for a "help" handler to be easily written.
type HandlerDocPair interface {
	Handler() HandlerFunc
	Name() string
	IsRegexpCommand() bool
	Regexp() *regexp.Regexp
	Description() string
	Usage() []string
	IsHidden() bool
	AliasNames() []string
	AddAliasName(string) bool
}

// HandlerDoc provides a base implementation of the HandlerDocPair interface.
type HandlerDoc struct {
	CmdHandler     HandlerFunc
	CmdName        string
	CmdDescription string
	CmdUsage       []string
	CmdIsHidden    bool
	cmdRegexp      *regexp.Regexp
	cmdAliasNames  []string
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

// Regexp returns the handler's set regexp. This should be nil for a normal
// command and not nil for a pattern command.
func (d *HandlerDoc) Regexp() *regexp.Regexp {
	return d.cmdRegexp
}

// IsRegexpCommand returns true if this command has a set regular expression
// or false if it's name should be used to match input to its handler.
func (d *HandlerDoc) IsRegexpCommand() bool {
	return d.cmdRegexp != nil
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

// AliasNames returns a sorted copy of the internal alias names slice. The
// returned slice is safe to modify and will never be nil although it could be
// a zero-length slice.
func (d *HandlerDoc) AliasNames() []string {
	aliasNamesCopy := make([]string, len(d.cmdAliasNames))
	copy(aliasNamesCopy, d.cmdAliasNames)
	return aliasNamesCopy
}

// AddAliasName adds a given alias name in sorted order to the internal alias
// names slice. This does not actually affect the dispatch and is only used
// for help text and to determine if an alias name has already been added.
//
// This returns true if the alias name was added and false if the given alias
// name is already set. This is case sensitive.
func (d *HandlerDoc) AddAliasName(aliasName string) bool {
	// binary search through sorted array to see if the alias name alraedy
	// exists
	pos := sort.SearchStrings(d.cmdAliasNames, aliasName)
	if len(d.cmdAliasNames) > 0 && pos < len(d.cmdAliasNames) && d.cmdAliasNames[pos] == aliasName {
		return false
	}
	d.cmdAliasNames = appendInOrderWithoutRepeats(d.cmdAliasNames, aliasName)
	return true
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
	regexpCommands []HandlerDocPair
	commandNames   []string
	patterns       []HandlerRegExpPair
	botNameRegex   *regexp.Regexp
	handlerMutex   *sync.RWMutex
}

// newDispatch returns a new *dispatch instance which matches all message
// routing methods specified in the victor.Robot interface.
func newDispatch(bot Robot) *dispatch {
	return &dispatch{
		robot:          bot,
		defaultHandler: nil,
		commands:       make(map[string]HandlerDocPair),
		botNameRegex:   regexp.MustCompile(fmt.Sprintf(botNameRegexFormat, bot.Name())),
		handlerMutex:   &sync.RWMutex{},
	}
}

// appendInOrderWithoutRepeats functions identically to the built-in "append"
// function except that it adds the given string in sorted order and will not
// add duplicates.
//
// This is safe to call with a nil array and is case sensitive.
func appendInOrderWithoutRepeats(array []string, toAdd string) []string {
	// find the insert using a binary search
	pos := sort.SearchStrings(array, toAdd)
	// check if we should add it
	if pos == len(array) || array[pos] != toAdd {
		// make space
		array = append(array, "")
		// move over existing data
		copy(array[pos+1:], array[pos:])
		// insert new element
		array[pos] = toAdd
	}
	return array
}

// Commands returns a copy of the internal commands map.
//
// This opens a read lock on the handlerMutex so new commands cannot be added
// while a copy of the commands is made (they will block until processing is
// completed)
func (d *dispatch) Commands() map[string]HandlerDocPair {
	d.handlerMutex.RLock()
	defer d.handlerMutex.RUnlock()
	cmdCopy := make(map[string]HandlerDocPair)
	for key, value := range d.commands {
		cmdCopy[key] = value
	}
	return cmdCopy
}

// EnableHelpCommand registers the default help handler with the bot's command
// map under the name "help". This will log a message if there is already a
// handler registered under that name.
//
// This opens a write lock on the handlerMutex or will wait until one can be
// opened. This is therefore safe to use concurrently with other handler
// functions and/or message processing.
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
//
// This opens a write lock on the handlerMutex or will wait until one can be
// opened. This is therefore safe to use concurrently with other handler
// functions and/or message processing.
func (d *dispatch) HandleCommand(cmd HandlerDocPair) {
	d.handlerMutex.Lock()
	defer d.handlerMutex.Unlock()
	lowerName := strings.ToLower(cmd.Name())
	if _, exists := d.commands[lowerName]; exists {
		log.Printf("\"%s\" has been set more than once.", lowerName)
	}
	newCmd := &HandlerDoc{
		CmdHandler:     cmd.Handler(),
		CmdName:        cmd.Name(),
		CmdDescription: cmd.Description(),
		CmdUsage:       cmd.Usage(),
		CmdIsHidden:    cmd.IsHidden(),
	}
	d.commands[lowerName] = newCmd
	d.commandNames = appendInOrderWithoutRepeats(d.commandNames, lowerName)
}

// HandleCommandPattern adds a given pattern to the bot's list of regexp
// commands. This is equivalent to calling HandleCommandRegexp but with a
// non-compiled regular expression.
//
// This uses regexp.MustCompile so it panics if given an invalid regular
// expression.
//
// This opens a write lock on the handlerMutex or will wait until one can be
// opened. This is therefore safe to use concurrently with other handler
// functions and/or message processing.
func (d *dispatch) HandleCommandPattern(pattern string, cmd HandlerDocPair) {
	d.HandleCommandRegexp(regexp.MustCompile(pattern), cmd)
}

// HandleCommandRegexp adds a given pattern to the bot's list of regular
// expression commands. These commands are then checked on any message that is
// considered a potential command (either sent @ the bot's name or in a direct
// message). They are evaluated only after no regular commands match the input
// and then they are checked in the same order as they were added. They are
// only checked against the first word of the message!
//
// This opens a write lock on the handlerMutex or will wait until one can be
// opened. This is therefore safe to use concurrently with other handler
// functions and/or message processing.
func (d *dispatch) HandleCommandRegexp(exp *regexp.Regexp, cmd HandlerDocPair) {
	d.handlerMutex.Lock()
	defer d.handlerMutex.Unlock()
	lowerName := strings.ToLower(cmd.Name())
	if exp == nil {
		log.Panicf("Cannot add nil regular expression command under name \"%s\"\n.", lowerName)
		return
	}
	newCmd := &HandlerDoc{
		CmdHandler:     cmd.Handler(),
		CmdName:        cmd.Name(),
		CmdDescription: cmd.Description(),
		CmdUsage:       cmd.Usage(),
		CmdIsHidden:    cmd.IsHidden(),
		cmdRegexp:      exp,
	}
	d.regexpCommands = append(d.regexpCommands, newCmd)
	d.commandNames = appendInOrderWithoutRepeats(d.commandNames, lowerName)
}

// HandleCommandAlias registers a given alias command name to the given
// existing command name. This will silently fail and output a log message
// if the given original command name does not exist or the new alias command
// name is equal to the original command name or the new alias command name is
// already set for the original command.
//
// This is equivalent to calling "HandleCommand" again for the alias command
// with the same documentation as the original command although this also
// registers the alias name with the original HandlerDocPair for help text
// purposes.
//
// This opens a write lock on the handlerMutex or will wait until one can be
// opened. This is therefore safe to use concurrently with other handler
// functions and/or message processing.
func (d *dispatch) HandleCommandAlias(originalName, aliasName string) {
	d.handlerMutex.Lock()
	lowerOrigName := strings.ToLower(originalName)
	doc, exists := d.commands[lowerOrigName]
	if !exists {
		log.Printf(`Cannot add alias for unset command "%s"`, lowerOrigName)
		return
	} else if strings.ToLower(originalName) == strings.ToLower(aliasName) {
		log.Printf(`A command cannot alias itself (command %s)`, lowerOrigName)
		return
	} else if !doc.AddAliasName(aliasName) {
		log.Printf(`Alias "%s" for original command "%s" already exists`, aliasName, lowerOrigName)
		return
	}
	newDoc := &HandlerDoc{
		CmdName:        aliasName,
		CmdIsHidden:    true,
		CmdHandler:     doc.Handler(),
		CmdDescription: doc.Description(),
		CmdUsage:       doc.Usage(),
	}
	// release our lock before actually adding the command
	d.handlerMutex.Unlock()
	d.HandleCommand(newDoc)
}

// HandleCommandAliasPattern adds a given pattern to the given commands aliases.
// This is equivalent to calling HandleCommandAliasRegexp but with a
// non-compiled regular expression.
//
// This uses regexp.MustCompile so it panics if given an invalid regular
// expression.
//
// This opens a write lock on the handlerMutex or will wait until one can be
// opened. This is therefore safe to use concurrently with other handler
// functions and/or message processing.
func (d *dispatch) HandleCommandAliasPattern(originalName, aliasName string, pattern string) {
	d.HandleCommandAliasRegexp(originalName, aliasName, regexp.MustCompile(pattern))
}

// HandleCommandAliasRegexp registers a given alias regexp to the given
// existing command name. This will silently fail and output a log message
// if the given original command name does not exist or the given regexp is nil.
// This will succeed but log a message if the given aliasName is already set.
//
// This is equivalent to calling "HandleCommandRegexp" again for the alias
// command regexp with the same documentation as the original command although
// this also registers the alias name with the original HandlerDocPair for
// help text purposes.
//
// The "aliasName" (second) parameter is used in order to list the alias in the
// help text for the original command. If this should be a "silent" (unlisted)
// alias then call this with the "aliasName" parameter as an empty string ("")
// and it will not be added.
//
// This opens a write lock on the handlerMutex or will wait until one can be
// opened. This is therefore safe to use concurrently with other handler
// functions and/or message processing.
func (d *dispatch) HandleCommandAliasRegexp(originalName, aliasName string, exp *regexp.Regexp) {
	d.handlerMutex.Lock()
	if exp == nil {
		log.Println("Cannot add nil regular expression.")
		return
	}
	lowerOrigName := strings.ToLower(originalName)
	doc, exists := d.commands[lowerOrigName]
	if !exists {
		log.Printf(`Cannot add alias for unset command "%s"`, lowerOrigName)
		return
	}
	if len(aliasName) > 0 && !doc.AddAliasName(aliasName) {
		log.Printf(
			`Alias "%s" for original command "%s" already exists - regexp was still added`,
			aliasName, lowerOrigName)
	}
	newDoc := &HandlerDoc{
		CmdName:        aliasName,
		CmdIsHidden:    true,
		CmdHandler:     doc.Handler(),
		CmdDescription: doc.Description(),
		CmdUsage:       doc.Usage(),
	}
	// release our lock before actually adding the alias
	d.handlerMutex.Unlock()
	d.HandleCommandRegexp(exp, newDoc)
}

// HandlePattern adds a given pattern to the bot's list of regexp expressions.
// This is equivalent to calling HandleRegexp but with a non-compiled regular
// expression.
//
// This uses regexp.MustCompile so it panics if given an invalid regular
// expression.
//
// This opens a write lock on the handlerMutex or will wait until one can be
// opened. This is therefore safe to use concurrently with other handler
// functions and/or message processing.
func (d *dispatch) HandlePattern(pattern string, handler HandlerFunc) {
	d.HandleRegexp(regexp.MustCompile(pattern), handler)
}

// HandleRegexp adds a given regular expression to the bot's list of regexp
// expressions. This expression will be checked on every message that is not
// considered a potential command (NOT sent @ the bot and NOT in a
// direct message). If multiple expressions are added then they will be
// evaluated in the order of insertion.
//
// This opens a write lock on the handlerMutex or will wait until one can be
// opened. This is therefore safe to use concurrently with other handler
// functions and/or message processing.
func (d *dispatch) HandleRegexp(exp *regexp.Regexp, handler HandlerFunc) {
	d.handlerMutex.Lock()
	defer d.handlerMutex.Unlock()
	if exp == nil {
		log.Println("Cannot add nil regular expression.")
		return
	}
	d.patterns = append(d.patterns, &handlerPair{
		exp:    exp,
		handle: handler,
	})
}

// SetDefaultHandler sets a function as the default handler which is called
// when a potential command message (either sent @ the bot's name or in a
// direct message) does not match any of the other set commands.
//
// This opens a write lock on the handlerMutex or will wait until one can be
// opened. This is therefore safe to use concurrently with other handler
// functions and/or message processing.
func (d *dispatch) SetDefaultHandler(handler HandlerFunc) {
	d.handlerMutex.Lock()
	defer d.handlerMutex.Unlock()
	if d.defaultHandler != nil {
		log.Println("Default handler has been set more than once.")
	}
	d.defaultHandler = handler
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
//
// This opens a read lock on the handlerMutex so new commands cannot be added
// while a message is being processed (they will block until processing is
// completed)
func (d *dispatch) ProcessMessage(m chat.Message) {
	d.handlerMutex.RLock()
	defer d.handlerMutex.RUnlock()
	defer func() {
		if e := recover(); e != nil {
			log.Println("Unexpected Panic Processing Message:", m.Text(), " -- Error:", e)
			return
		}
	}()
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

// callDefault invokes the default message handler if one is set.
// If one is not set then it logs the unhandled occurrance but otherwise does
// not fail.
//
// This does not acquire a lock on the handlerMutex but one should be acquired
// for reading before calling this method.
func (d *dispatch) callDefault(m chat.Message, messageText string) {
	fields := parseFields(messageText)
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
// with the bot's name and any follwing text up until the next word whitespace
// removed. This returns true if a match is made and false otherwise.
//
// This does not acquire a lock on the handlerMutex but one should be acquired
// for reading before calling this method.
func (d *dispatch) matchCommands(m chat.Message, messageText string) bool {
	fullFields := parseFields(messageText)
	if len(fullFields) == 0 {
		return false
	}
	commandName := strings.ToLower(fullFields[0])
	fields := fullFields[1:]
	command, defined := d.commands[commandName]
	if !defined || command.IsRegexpCommand() {
		return d.matchCommandRegexp(m, messageText, commandName, fields)
	}
	command.Handler().Handle(&state{
		robot:   d.robot,
		message: m,
		fields:  fields,
	})
	return true
}

// matchCommandRegexp attemps to match a given command word from the given
// message to one of the added regular expression commands. It expects the
// second parameter to be the message's text with the bot's name and any
// following whitespace removed. This returns true if a match is made and false
// otherwise.
//
// This performs a linear search through the slice of regular expression
// commands so their priority is the same as the insertion order.
//
// This does not acquire a lock on the handlerMutex but one should be acquired
// for reading before calling this method.
func (d *dispatch) matchCommandRegexp(m chat.Message, messageText, commandName string, fields []string) bool {
	cmd := d.findCommandRegexp(commandName)
	if cmd == nil {
		return false
	}
	cmd.Handler().Handle(&state{
		robot:   d.robot,
		message: m,
		fields:  fields,
	})
	return true

}

// findCommandRegexp searches the dispatch's internal slice of regexpCommands
// by attempting to match the given string to all registered regexp commands.
// It does this by performing a linear search through the slice and therefore
// searches in order of insertion. This is safe to call if the internal slice
// of command regexps is nil.
//
// This does not acquire a lock on the handlerMutex but one should be acquired
// for reading before calling this method.
func (d *dispatch) findCommandRegexp(commandPart string) HandlerDocPair {
	for _, cmd := range d.regexpCommands {
		if cmd.Regexp().MatchString(commandPart) {
			return cmd
		}
	}
	return nil
}

// matchPatterns iterates through the array of registered regular expressions
// (patterns) in the order of insertion and checks if they match any part of
// the given message's text. If they do then they are invoked with an empty
// fields array.
//
// This does not acquire a lock on the handlerMutex but one should be acquired
// for reading before calling this method.
func (d *dispatch) matchPatterns(m chat.Message) bool {
	for _, pair := range d.patterns {
		if pair.Exp().MatchString(m.Text()) {
			pair.Handler().Handle(&state{
				robot:   d.robot,
				message: m,
			})
			return true
		}
	}
	return false
}

// defaultHelpHandler either shows all available (and non-hidden) commands or
// the help text for a given command.
//
// This opens a read lock on the handlerMutex so new commands cannot be added
// while a message is being processed (they will block until processing is
// completed)
func defaultHelpHandler(s State, d *dispatch) {
	d.handlerMutex.RLock()
	defer d.handlerMutex.RUnlock()
	if len(s.Fields()) == 0 {
		showAllCommands(s, d)
	} else {
		showCommandHelp(s, d)
	}
}

// showAllCommands is used by the default help handler to show a list of all
// non-hidden commands regsitered to the dispatch.
//
// This does not acquire a lock on the handlerMutex but one should be acquired
// for reading before calling this method.
func showAllCommands(s State, d *dispatch) {
	if len(d.commandNames) == 0 {
		s.Chat().Send(s.Message().Channel().ID(), "No commands have been set!")
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
		buf.WriteString(fmt.Sprintf("*%s*", docPair.Name()))
		if len(docPair.Description()) > 0 {
			buf.WriteString(fmt.Sprintf(" - _%s_", docPair.Description()))
		}
		buf.WriteString("\n")
	}
	buf.WriteString("\nFor help with a command, type `help [command name]`.")
	s.Reply(buf.String())
}

// showCommandHelp shows the description, usage, and aliases if they are set
// for a given command name. The command name should be the first element in
// the state's Fields.
//
// This does not acquire a lock on the handlerMutex but one should be acquired
// for reading before calling this method.
func showCommandHelp(s State, d *dispatch) {
	if len(s.Fields()) == 0 {
		return
	}
	cmdName := strings.ToLower(s.Fields()[0])
	docPair, exists := d.commands[cmdName]
	if !exists {
		docPair = d.findCommandRegexp(cmdName)
		if docPair == nil {
			textFmt := "Unrecognized command _%s_.  Type *`help`* to view a list of all available commands."
			s.Chat().Send(s.Message().Channel().ID(), fmt.Sprintf(textFmt, cmdName))
			return
		}
	}
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("*%s*", cmdName))
	if len(docPair.Description()) > 0 {
		buf.WriteString(fmt.Sprintf(" - _%s_", docPair.Description()))
	}
	buf.WriteString("\n\n")
	aliasNames := docPair.AliasNames()
	if len(aliasNames) > 0 {
		buf.WriteString("Alias: _")
		for i := range aliasNames {
			buf.WriteString(aliasNames[i])
			if i+1 < len(aliasNames) {
				buf.WriteString(", ")
			}
		}
		buf.WriteString("_\n")
	}
	if len(docPair.Usage()) > 0 {
		buf.WriteString(">>>")
		for _, use := range docPair.Usage() {
			buf.WriteString(cmdName)
			buf.WriteString(" ")
			buf.WriteString(use)
			buf.WriteString("\n")
		}
	}
	s.Reply(buf.String())
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
