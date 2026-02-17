package context

type UserInput struct {
	Channel string // channel source
	ChatId  string // chat id
	Content string // user input content
	Created int64  // created at unix timestamp
}
