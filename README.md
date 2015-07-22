## Victor

**Victor** is a library for creating your own chat bot.

Victor is a fork of [brettbuddin/victor](https://github.com/brettbuddin/victor) with several breaking changes to routing.

### Supported Services

Currently there is a chat adatper for the [Slack Real Time API](https://api.slack.com/rtm). There are other adapters on the [original repo](https://github.com/brettbuddin/victor) that will need some modification due to the breaking changes of this fork.

One breaking change is the addition of two chat event-driven channels which are handled by the Robot interface in the methods `robot.ChatErrors()` and `robot.ChatEvents()`. These channels must be "listened" to or over time there will be many goroutines waiting on blocking sends. The Slack Real Time adapter is designed such that all sends to these channels are performed on goroutines and will therefore continue to work. Ignoring the channels is not recommended and simply receiving the events pushed to them and ignoring them will suffice. For an example, look at the [examples](https://github.com/FogCreek/victor/tree/master/examples).

*   **Slack Real Time**
    To use victor with the slack real time adapter, you need to [add a new bot](https://my.slack.com/services/new/bot) and initialize victor with an adapterConfig struct that matches the victor/pkg/chat/slackRealtime.Config interface to return its token.

    At the moment the bot's `Stop` method is broken with this adapter!
    

A simple example is located in [examples](https://github.com/FogCreek/victor/tree/master/examples).
