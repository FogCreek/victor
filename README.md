## Victor

**Victor** is a library for creating your own chat bot.

Victor is a fork of [brettbuddin/victor](https://github.com/brettbuddin/victor) with several breaking changes to routing.

### Supported Services

Currently there is a chat adatper for the [Slack Real Time API](https://api.slack.com/rtm). There are other adapters on the [original repo](https://github.com/brettbuddin/victor) that will need some modification due to the breaking changes of this fork.

*   **Slack Real Time**
    To use victor with the slack real time adapter, you need to [add a new bot](https://michaelsfogbugztest.slack.com/services/new/bot) and initialize victor with an adapterConfig struct that matches the victor/pkg/chat/slackRealtime.Config interface to return its token.

    At the moment the bot's `Stop` method is broken with this adapter!
    

A simple example is located in `examples/`.
