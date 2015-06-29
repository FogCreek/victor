package chat

// BaseUser provides a bare set/get implementation of the chat.User
// interface that can be used by an adapter if it requires no additional logic
// in its Users.
type BaseUser struct {
	UserID,
	UserName,
	UserEmail string
	UserIsBot bool
}

// ID returns the User's chat ID.
func (u *BaseUser) ID() string {
	return u.UserID
}

// Name returns the User's full name.
func (u *BaseUser) Name() string {
	return u.UserName
}

// EmailAddress returns the user's email address in string form.
func (u *BaseUser) EmailAddress() string {
	return u.UserEmail
}

// IsBot returns true if the user is a bot and false otherwise.
func (u *BaseUser) IsBot() bool {
	return u.UserIsBot
}
