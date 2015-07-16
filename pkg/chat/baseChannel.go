package chat

type BaseChannel struct {
	ChannelID,
	ChannelName string
}

func (b *BaseChannel) ID() string {
	return b.ChannelID
}

func (b *BaseChannel) Name() string {
	return b.ChannelName
}
