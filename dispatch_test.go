package victor

import (
	"testing"

	"github.com/FogCreek/victor/pkg/chat"
	_ "github.com/FogCreek/victor/pkg/chat/mockAdapter"

	"github.com/stretchr/testify/assert"
)

const botName = "@testBot"

// Returns a new *robot using the "mockAdapter" chat adapter and the bot name
// set in the "botName" constant.
func getMockBot() *robot {
	return New(Config{
		Name:        botName[1:],
		ChatAdapter: "mockAdapter",
	})
}

// HandlerMock provides an easy way to construct a handler and assert how many
// times it has been called by the message router and/or the fields that it is
// called with.
type HandlerMock struct {
	t              *testing.T
	timesRun       int
	expectedFields []string
}

// TimesRun returns the number of times that the function returned by this
// HandlerMock's Func() method has been called.
func (h *HandlerMock) TimesRun() int {
	return h.timesRun
}

// Func returns a function of type HandlerFunc which increments an internal
// counter every time it is called. Multiple calls to Func() will return
// different function instances but all will increment the same internal count.
//
// If the expected fields property is set to a non-nil value then a call to the
// returned function will assert the expected and actual fields equality.
func (h *HandlerMock) Func() HandlerFunc {
	return func(s State) {
		h.timesRun++
		if h.expectedFields != nil {
			assert.Equal(h.t, h.expectedFields, s.Fields(), "Fields mismatch - fields incorrectly parsed.")
		}
	}
}

// HasRun asserts that this HandlerMock instance has been run the expected
// number of times. This is equivalent to HasRunCustom but with a preset failed
// message.
func (h *HandlerMock) HasRun(expectedTimesRun int) {
	h.HasRunCustom(expectedTimesRun, "Count mismatch - handlers incorrectly called.")
}

// HasRunCustom asserts that this HandlerMock instance has been run the expected
// number of times. The "failedMessage" parameter is the message that will be
// shown if the assertion fails.
func (h *HandlerMock) HasRunCustom(expectedTimesRun int, failedMessage string) {
	if expectedTimesRun != h.timesRun {
		assert.Fail(h.t, failedMessage)
	}
	// assert.Equal(h.t, expectedTimesRun, h.timesRun, failedMessage)
}

// ExpectFields sets up an assertion that the fields returned by the State
// object upon a call to the handler are equal to the string slice given.
// This overrides any previous expected fields. If this is set to nil then the
// fields will not be checked on a call to the handler function.
func (h *HandlerMock) ExpectFields(fields []string) {
	h.expectedFields = fields
}

func TestNewDispatch(t *testing.T) {
	bot := getMockBot()
	assert.Empty(t, bot.dispatch.commands, "A new bot should have no commands.")
	assert.Nil(t, bot.dispatch.defaultHandler, "Default handler should be nil.")
	assert.Empty(t, bot.commandNames, "No command names should be stored on creation.")
	assert.Empty(t, bot.patterns, "Patterns array should be empty on creation.")
}

func TestEnableHelp(t *testing.T) {
	bot := getMockBot()
	bot.dispatch.EnableHelpCommand()
	assert.Len(t, bot.dispatch.commands, 1, "Help handler should have been added once.")
	assert.Len(t, bot.dispatch.commandNames, 1, "Help handler should be in command names.")
	assert.Equal(t, helpCommandName, bot.dispatch.commandNames[0], "Help handler should be in command names.")
	for key := range bot.dispatch.commands {
		assert.Equal(t, helpCommandName, key, "Help handler command should be \"help\".")
	}
}

func TestCommandsGetter(t *testing.T) {
	bot := getMockBot()
	bot.dispatch.HandleCommand(&HandlerDoc{CmdName: "test"})
	mapCopy := bot.dispatch.Commands()
	assert.Equal(t, mapCopy, bot.dispatch.commands, "Map copy should be equal")
	mapCopy["mock"] = nil
	assert.NotEqual(t, mapCopy, bot.dispatch.commands, "Map copy should be a copy of original commands map")
}

func TestHandleCommand(t *testing.T) {
	bot := getMockBot()
	handler := HandlerMock{t: t}
	bot.dispatch.HandleCommand(&HandlerDoc{
		CmdHandler:     handler.Func(),
		CmdName:        "name",
		CmdUsage:       []string{"", "1", "2"},
		CmdDescription: "description",
		CmdIsHidden:    true,
	})
	handler.HasRunCustom(0, "Handler should not have been called on creation.")
	assert.Len(t, bot.dispatch.commands, 1, "Added command should be present in map.")
	assert.Len(t, bot.dispatch.commandNames, 1, "Added command should be in commandNames list.")
	// testify.assert.Contains doesn't support map keys right now https://github.com/stretchr/testify/pull/165
	actualHandlerFunc, exists := bot.dispatch.commands["name"]
	assert.True(t, exists, "Bot should contain new handler in commands map.")
	assert.Contains(t, bot.dispatch.commandNames, "name", "Bot should contain new handler in commandNames")
	assert.Equal(t, "name", actualHandlerFunc.Name(), "HandlerDoc name changed.")
	assert.Equal(t, []string{"", "1", "2"}, actualHandlerFunc.Usage(), "HandlerDoc usage changed.")
	assert.Equal(t, "description", actualHandlerFunc.Description(), "HandlerDoc description changed.")
	assert.True(t, actualHandlerFunc.IsHidden(), "HandlerDoc IsHidden property changed.")
	actualHandlerFunc.Handler().Handle(nil)
	handler.HasRunCustom(1, "Handler function should have increased count on call to Handle")
}

func TestProcessMessageCommand(t *testing.T) {
	bot := getMockBot()
	name0Handle := HandlerMock{t: t}
	name1Handle := HandlerMock{t: t}
	bot.dispatch.HandleCommand(&HandlerDoc{
		CmdHandler: name0Handle.Func(),
		CmdName:    "name0",
	})
	bot.dispatch.HandleCommand(&HandlerDoc{
		CmdHandler: name1Handle.Func(),
		CmdName:    "name1",
	})
	// by default will not be in a direct message unless specified otherwise
	// should not call a handler
	bot.dispatch.ProcessMessage(&chat.BaseMessage{MsgText: "name0"})
	name0Handle.HasRunCustom(0, "Handler should not have been called yet.")
	name1Handle.HasRunCustom(0, "Handler should not have been called yet.")
	// should call "name0" handler
	bot.dispatch.ProcessMessage(&chat.BaseMessage{MsgText: "name0", MsgIsDirect: true})
	name0Handle.HasRunCustom(1, "\"name0\" handler should have been called")
	name1Handle.HasRunCustom(0, "\"name1\" handler should not have been called")
	// should call "name0" handler
	bot.dispatch.ProcessMessage(&chat.BaseMessage{MsgText: botName + " name0"})
	name0Handle.HasRun(2)
	name1Handle.HasRun(0)
	// should call "name1" handler
	bot.dispatch.ProcessMessage(&chat.BaseMessage{MsgText: botName + "name1 param"})
	name0Handle.HasRun(2)
	name1Handle.HasRun(1)
}

func TestFieldsDirectly(t *testing.T) {
	var expectedOutput []string
	var input string

	expectedOutput = []string{}
	input = ""
	assert.Equal(t, expectedOutput, parseFields(input), "Incorrect field parsing.")

	expectedOutput = []string{}
	input = "    \t\t\n   "
	assert.Equal(t, expectedOutput, parseFields(input), "Incorrect field parsing.")

	expectedOutput = []string{"a"}
	input = "a"
	assert.Equal(t, expectedOutput, parseFields(input), "Incorrect field parsing.")

	expectedOutput = []string{"a", "b"}
	input = "a     b"
	assert.Equal(t, expectedOutput, parseFields(input), "Incorrect field parsing.")

	expectedOutput = []string{"a", "b"}
	input = "a\t\t\n   b"
	assert.Equal(t, expectedOutput, parseFields(input), "Incorrect field parsing.")

	expectedOutput = []string{"a\t  b"}
	input = "\"a\t  b\""
	assert.Equal(t, expectedOutput, parseFields(input), "Incorrect field parsing.")

	input = "you're it's \"i'm who's\""
	expectedOutput = []string{"you're", "it's", "i'm who's"}
	assert.Equal(t, expectedOutput, parseFields(input), "Incorrect field parsing.")

	input = "\"test of\" \t\n       unclosed\t \"quotes and\tspaces"
	expectedOutput = []string{"test of", "unclosed", "quotes and\tspaces"}
	assert.Equal(t, expectedOutput, parseFields(input), "Incorrect field parsing.")
}

func TestFieldsThroughBot(t *testing.T) {
	msg := &chat.BaseMessage{MsgIsDirect: true}
	bot := getMockBot()
	handler := HandlerMock{t: t}
	bot.dispatch.HandleCommand(&HandlerDoc{
		CmdHandler: handler.Func(),
		CmdName:    "test",
	})

	handler.ExpectFields([]string{})
	msg.MsgText = "test"
	bot.ProcessMessage(msg)
	handler.HasRun(1)

	handler.ExpectFields([]string{"test"})
	msg.MsgText = "test \t\n test"
	bot.ProcessMessage(msg)
	handler.HasRun(2)

	handler.ExpectFields([]string{"this", "is", "a", "test"})
	msg.MsgText = "test this is a test"
	bot.ProcessMessage(msg)
	handler.HasRun(3)

	handler.ExpectFields([]string{"this", " is a test"})
	msg.MsgText = "test this \" is a test\""
	bot.ProcessMessage(msg)
	handler.HasRun(4)
}

func TestFieldsDefaultHandler(t *testing.T) {
	msg := &chat.BaseMessage{MsgIsDirect: true}
	bot := getMockBot()
	handler := HandlerMock{t: t}
	bot.dispatch.SetDefaultHandler(handler.Func())

	handler.ExpectFields([]string{})
	msg.MsgText = ""
	bot.ProcessMessage(msg)
	handler.HasRun(1)

	handler.ExpectFields([]string{"test"})
	msg.MsgText = "test"
	bot.ProcessMessage(msg)
	handler.HasRun(2)

	handler.ExpectFields([]string{"test", "this", "is", "a", "test"})
	msg.MsgText = "test this is a test"
	bot.ProcessMessage(msg)
	handler.HasRun(3)

	handler.ExpectFields([]string{"test", "this", " is a test"})
	msg.MsgText = "test this \" is a test\""
	bot.ProcessMessage(msg)
	handler.HasRun(4)

	handler.ExpectFields([]string{"123"})
	msg.MsgText = botName + " 123"
	bot.ProcessMessage(msg)
	handler.HasRun(5)
}

func TestDefaultHandler(t *testing.T) {
	bot := getMockBot()
	defaultHandle := HandlerMock{t: t}
	otherHandle := HandlerMock{t: t}
	bot.dispatch.HandleCommand(&HandlerDoc{
		CmdHandler: otherHandle.Func(),
		CmdName:    "test",
	})
	bot.dispatch.SetDefaultHandler(defaultHandle.Func())
	msg := &chat.BaseMessage{MsgIsDirect: true}
	// should call default handler
	bot.ProcessMessage(msg)
	defaultHandle.HasRun(1)
	otherHandle.HasRun(0)
	msg.MsgText = "test"
	// should not call default handler but should call other handler
	bot.ProcessMessage(msg)
	defaultHandle.HasRun(1)
	otherHandle.HasRun(1)
	msg.MsgText = "asdf"
	msg.MsgIsDirect = false
	// should not call any handlers
	bot.ProcessMessage(msg)
	defaultHandle.HasRun(1)
	otherHandle.HasRun(1)
	msg.MsgText = botName + " asdf"
	// should call default handler even though not direct
	bot.ProcessMessage(msg)
	defaultHandle.HasRun(2)
	otherHandle.HasRun(1)
}

func TestPatterns(t *testing.T) {
	bot := getMockBot()
	commandHandle := HandlerMock{t: t}
	patternHandle := HandlerMock{t: t}
	defaultHandle := HandlerMock{t: t}
	bot.HandleCommand(&HandlerDoc{
		CmdHandler: commandHandle.Func(),
		CmdName:    "pattern",
	})
	// set up known pattern
	// case insensitive match for word "pattern" or "patterns"
	bot.HandlePattern("(?i)\\s*pattern[s]?\\s*", patternHandle.Func())
	bot.SetDefaultHandler(defaultHandle.Func())

	defaultHandle.HasRun(0)
	commandHandle.HasRun(0)
	patternHandle.HasRun(0)
	msg := &chat.BaseMessage{MsgIsDirect: false}
	msg.MsgText = "pattern"
	// should fire pattern and not cmd or defualt
	bot.ProcessMessage(msg)
	defaultHandle.HasRun(0)
	commandHandle.HasRun(0)
	patternHandle.HasRun(1)
	msg.MsgIsDirect = true
	// should fire on cmd and not pattern
	bot.ProcessMessage(msg)
	defaultHandle.HasRun(0)
	commandHandle.HasRun(1)
	patternHandle.HasRun(1)
	msg.MsgText = "patterns"
	// should fire default handler
	bot.ProcessMessage(msg)
	defaultHandle.HasRun(1)
	commandHandle.HasRun(1)
	patternHandle.HasRun(1)
	msg.MsgIsDirect = false
	// should fire pattern
	bot.ProcessMessage(msg)
	defaultHandle.HasRun(1)
	commandHandle.HasRun(1)
	patternHandle.HasRun(2)
	msg.MsgText = "test for the_PaTTeRNs_handler"
	// should still match the pattern
	bot.ProcessMessage(msg)
	defaultHandle.HasRun(1)
	commandHandle.HasRun(1)
	patternHandle.HasRun(3)
}

func TestCommandPatternsFiring(t *testing.T) {
	bot := getMockBot()
	patternHandle := HandlerMock{t: t}
	commandHandle := HandlerMock{t: t}
	defaultHandle := HandlerMock{t: t}
	// set up pattern command
	// matches "hank", "hanks", thank", and "thanks" case insensitively
	bot.HandleCommandPattern("(?i)[t]?hank[s]?", &HandlerDoc{
		CmdHandler: patternHandle.Func(),
		CmdName:    "pattern",
	})
	bot.HandleCommand(&HandlerDoc{
		CmdHandler: commandHandle.Func(),
		CmdName:    "thanks",
	})
	bot.SetDefaultHandler(defaultHandle.Func())
	msg := &chat.BaseMessage{}

	msg.MsgIsDirect = false
	msg.MsgText = "thank you"
	// should not fire command pattern or default
	bot.ProcessMessage(msg)
	defaultHandle.HasRun(0)
	patternHandle.HasRun(0)
	commandHandle.HasRun(0)

	msg.MsgIsDirect = true
	msg.MsgText = "pattern"
	// should fire default handler
	bot.ProcessMessage(msg)
	defaultHandle.HasRun(1)
	patternHandle.HasRun(0)
	commandHandle.HasRun(0)

	msg.MsgText = "thanks"
	// should fire command and not pattern or default
	bot.ProcessMessage(msg)
	defaultHandle.HasRun(1)
	patternHandle.HasRun(0)
	commandHandle.HasRun(1)

	msg.MsgIsDirect = false
	msg.MsgText = botName + " thank you"
	// should fire command pattern
	bot.ProcessMessage(msg)
	defaultHandle.HasRun(1)
	patternHandle.HasRun(1)
	commandHandle.HasRun(1)

	msg.MsgText = botName + "than you"
	// should fire default handler
	bot.ProcessMessage(msg)
	defaultHandle.HasRun(2)
	patternHandle.HasRun(1)
	commandHandle.HasRun(1)

	msg.MsgIsDirect = true
	msg.MsgText = "ThAnk \tyou field1 field2"
	// should fire command pattern
	bot.ProcessMessage(msg)
	defaultHandle.HasRun(2)
	patternHandle.HasRun(2)
	commandHandle.HasRun(1)

	msg.MsgIsDirect = true
	msg.MsgText = "abcd thank you efg"
	// should fire default handler despite match in middle of string
	bot.ProcessMessage(msg)
	defaultHandle.HasRun(3)
	patternHandle.HasRun(2)
	commandHandle.HasRun(1)
}

func TestCommandPatternsFields(t *testing.T) {
	bot := getMockBot()
	handler := HandlerMock{t: t}
	// matches "hank", "hanks", thank", and "thanks" case insensitively
	bot.HandleCommandPattern("(?i)[t]?hank[s]?", &HandlerDoc{
		CmdHandler: handler.Func(),
		CmdName:    "pattern",
	})
	msg := &chat.BaseMessage{MsgIsDirect: true}

	msg.MsgText = "thanks"
	handler.ExpectFields([]string{})
	bot.ProcessMessage(msg)
	handler.HasRun(1)

	msg.MsgText = "thanks\t\t\n  "
	handler.ExpectFields([]string{})
	bot.ProcessMessage(msg)
	handler.HasRun(2)

	msg.MsgText = "thank s a bunch"
	handler.ExpectFields([]string{"s", "a", "bunch"})
	bot.ProcessMessage(msg)
	handler.HasRun(3)

	msg.MsgText = "ThAnkyou for \"every thing\""
	handler.ExpectFields([]string{"for", "every thing"})
	bot.ProcessMessage(msg)
	handler.HasRun(4)
}
