package victor

import (
	"testing"

	"github.com/FogCreek/victor/pkg/chat"
	_ "github.com/FogCreek/victor/pkg/chat/mockAdapter"

	"github.com/stretchr/testify/assert"
)

const botName = "testBot"

// Returns a new *robot using the "mockAdapter" chat adapter and the bot name
// set in the "botName" constant.
func getMockBot() *robot {
	return New(Config{
		Name:        botName,
		ChatAdapter: "mockAdapter",
	})
}

// HandlerCount provides an easy way to construct a handler and assert how many
// times it has been called by the message router.
type HandlerCount struct {
	t        *testing.T
	timesRun int
}

// TimesRun returns the number of times that the function returned by this
// HandlerCount's Func() method has been called.
func (h *HandlerCount) TimesRun() int {
	return h.timesRun
}

// Func returns a function of type HandlerFunc which increments an internal
// counter every time it is called. Multiple calls to Func() will return
// different function instances but all will increment the same internal count.
func (h *HandlerCount) Func() HandlerFunc {
	return func(s State) {
		h.timesRun++
	}
}

// HasRun asserts that this HandlerCount instance has been run the expected
// number of times. This is equivalent to HasRunCustom but with a preset failed
// message.
func (h *HandlerCount) HasRun(expectedTimesRun int) {
	h.HasRunCustom(expectedTimesRun, "Count mismatch - handlers incorrectly called.")
}

// HasRunCustom asserts that this HandlerCount instance has been run the expected
// number of times. The "failedMessage" parameter is the message that will be
// shown if the assertion fails.
func (h *HandlerCount) HasRunCustom(expectedTimesRun int, failedMessage string) {
	if expectedTimesRun != h.timesRun {
		assert.Fail(h.t, failedMessage)
	}
	// assert.Equal(h.t, expectedTimesRun, h.timesRun, failedMessage)
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
	var handlerFunc HandlerFunc
	count := 0
	handlerFunc = func(s State) {
		if count == 0 {
			assert.Fail(t, "Added handler should not have been called upon creation.")
		}
		// should make count == 2 when called for the first time
		count++
	}
	bot.dispatch.HandleCommand(&HandlerDoc{
		CmdHandler:     handlerFunc,
		CmdName:        "name",
		CmdUsage:       []string{"", "1", "2"},
		CmdDescription: "description",
		CmdIsHidden:    true,
	})
	// should make count == 1 so handlerFunc doesn't fail
	count++
	assert.Len(t, bot.dispatch.commands, 1, "Added command should be present in map.")
	assert.Len(t, bot.dispatch.commandNames, 1, "Added command should be in commandNames list.")
	// testify.assert.Contains doesn't support map keys right now https://github.com/stretchr/testify/pull/165
	actualHandlerFunc, exists := bot.dispatch.commands["name"]
	assert.True(t, exists, "Bot should contain new handler in commands map.")
	assert.Contains(t, bot.dispatch.commandNames, "name", "Bot should contain new handler in commandNames")
	actualHandlerFunc.Handler().Handle(nil)
	assert.Equal(t, 2, count, "Handler function should have increased count on call to Handle")
}

func TestProcessMessageCommand(t *testing.T) {
	bot := getMockBot()
	name0Handle := HandlerCount{t: t}
	name1Handle := HandlerCount{t: t}
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
	var expectedFields []string
	msg := &chat.BaseMessage{MsgIsDirect: true}
	bot := getMockBot()
	handler := func(s State) {
		assert.Equal(t, expectedFields, s.Fields(), "Fields mismatch.")
	}
	bot.dispatch.HandleCommand(&HandlerDoc{
		CmdHandler: handler,
		CmdName:    "test",
	})
	expectedFields = []string{}
	msg.MsgText = "test"
	bot.ProcessMessage(msg)
	expectedFields = []string{"test"}
	msg.MsgText = "test \t\n test"
	bot.ProcessMessage(msg)
	expectedFields = []string{"this", "is", "a", "test"}
	msg.MsgText = "test this is a test"
	bot.ProcessMessage(msg)
	expectedFields = []string{"this", " is a test"}
	msg.MsgText = "test this \" is a test\""
	bot.ProcessMessage(msg)
}

func TestFieldsDefaultHandler(t *testing.T) {
	var expectedFields []string
	msg := &chat.BaseMessage{MsgIsDirect: true}
	bot := getMockBot()
	handler := func(s State) {
		assert.Equal(t, expectedFields, s.Fields(), "Fields mismatch.")
	}
	bot.dispatch.SetDefaultHandler(handler)
	expectedFields = []string{}
	msg.MsgText = ""
	bot.ProcessMessage(msg)
	expectedFields = []string{"test"}
	msg.MsgText = "test"
	bot.ProcessMessage(msg)
	expectedFields = []string{"test", "this", "is", "a", "test"}
	msg.MsgText = "test this is a test"
	bot.ProcessMessage(msg)
	expectedFields = []string{"test", "this", " is a test"}
	msg.MsgText = "test this \" is a test\""
	bot.ProcessMessage(msg)

	expectedFields = []string{"123"}
	msg.MsgText = botName + " 123"
	bot.ProcessMessage(msg)
}

func TestDefaultHandler(t *testing.T) {
	bot := getMockBot()
	defaultHandle := HandlerCount{t: t}
	otherHandle := HandlerCount{t: t}
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
	commandHandle := HandlerCount{t: t}
	patternHandle := HandlerCount{t: t}
	defaultHandle := HandlerCount{t: t}
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
