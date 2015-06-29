package victor

import (
	"testing"

	"github.com/FogCreek/victor/pkg/chat"
	_ "github.com/FogCreek/victor/pkg/chat/mockAdapter"

	"github.com/stretchr/testify/assert"
)

const botName = "testBot"

func getMockBot() *robot {
	return New(Config{
		Name:        botName,
		ChatAdapter: "mockAdapter",
	})
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
	count0 := 0
	count1 := 0
	bot.dispatch.HandleCommand(&HandlerDoc{
		CmdHandler: func(s State) { count0++ },
		CmdName:    "name0",
	})
	bot.dispatch.HandleCommand(&HandlerDoc{
		CmdHandler: func(s State) { count1++ },
		CmdName:    "name1",
	})

	check := func(count0Exp, count1Exp int) {
		assert.Equal(t, count0Exp, count0, "Count mismatch - handlers incorrectly called.")
		assert.Equal(t, count1Exp, count1, "Count mismatch - handlers incorrectly called.")
	}
	// by default will not be in a direct message unless specified otherwise
	// should not call a handler
	bot.dispatch.ProcessMessage(&chat.BaseMessage{MsgText: "name0"})
	check(0, 0)
	// should call "name0" handler
	bot.dispatch.ProcessMessage(&chat.BaseMessage{MsgText: "name0", MsgIsDirect: true})
	check(1, 0)
	// should call "name0" handler
	bot.dispatch.ProcessMessage(&chat.BaseMessage{MsgText: botName + " name0"})
	check(2, 0)
	// should call "name1" handler
	bot.dispatch.ProcessMessage(&chat.BaseMessage{MsgText: botName + "name1 param"})
	check(2, 1)
}

func TestFields(t *testing.T) {
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

// TODO test default handler
// TODO test patterns
