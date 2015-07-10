package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/FogCreek/victor"
)

const BOT_NAME = "victor"

func main() {
	bot := victor.New(victor.Config{
		ChatAdapter: "shell",
		Name:        BOT_NAME,
	})
	addHandlers(bot)
	// optional help built in help command
	bot.EnableHelpCommand()
	err := bot.Run()
	if err != nil {
		log.Println(err.Error())
	}
	// keep the process (and bot) alive
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)
	<-sigs

	bot.Stop()
}

func addHandlers(r victor.Robot) {
	// Add a typical command that will be displayed using the "help" command
	// if it is enabled.
	r.HandleCommand(&victor.HandlerDoc{
		CmdHandler:     byeFunc,
		CmdName:        "hi",
		CmdDescription: "Says goodbye when the user says hi!",
		CmdUsage:       []string{""},
	})
	// Add a hidden command that isn't displayed in the "help" command unless
	// mentioned by name
	r.HandleCommand(&victor.HandlerDoc{
		CmdHandler:     echoFunc,
		CmdName:        "echo",
		CmdDescription: "Hidden `echo` command!",
		CmdUsage:       []string{"", "`text to echo`"},
		CmdIsHidden:    true,
	})
	// Add a command to show the "Fields" method
	r.HandleCommand(&victor.HandlerDoc{
		CmdHandler:     fieldsFunc,
		CmdName:        "fields",
		CmdDescription: "Show the fields/parameters of a command message!",
		CmdUsage:       []string{"`param0` `param1` `...`"},
	})
	// Add a general pattern which is only checked on "non-command" messages
	// which are described in dispatch.go
	r.HandlePattern("\b(thanks|thank\\s+you)\b", thanksFunc)
	// Add default handler to show "unrecognized command" on "command" messages
	r.SetDefaultHandler(defaultFunc)
}

func byeFunc(s victor.State) {
	msg := fmt.Sprintf("Bye %s!", s.Message().User().Name())
	s.Chat().Send(s.Message().ChannelID(), msg)
}

func echoFunc(s victor.State) {
	s.Chat().Send(s.Message().ChannelID(), s.Message().Text())
}

func thanksFunc(s victor.State) {
	msg := fmt.Sprintf("You're welcome %s!", s.Message().User().Name())
	s.Chat().Send(s.Message().ChannelID(), msg)
}

func fieldsFunc(s victor.State) {
	var fStr string
	for _, f := range s.Fields() {
		fStr += f + "\n"
	}
	s.Chat().Send(s.Message().ChannelID(), fStr)
}

func defaultFunc(s victor.State) {
	s.Chat().Send(s.Message().ChannelID(),
		"Unrecognized command. Type `help` to see supported commands.")
}
