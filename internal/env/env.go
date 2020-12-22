package env

import "os"

// SlackToken holds the Slack app token, like xoxo-123980123981237891273891273921
func SlackToken() string {
	return os.Getenv("SLACK_TOKEN")
}

// ChannelID is the channel id to post the message, usually like C636276209
func ChannelID() string {
	return os.Getenv("CHANNEL_ID")
}

// RunID is the GH run id - just pass this in from the Github context
func RunID() string {
	return os.Getenv("RUN_ID")
}

// Tenant is whatever you want to call your environment - dev, qa, prod, etc
func Tenant() string {
	return os.Getenv("TENANT")
}

// RepoURL is the URL to your GH repo, like https://github.com/myuser/myrepo
func RepoURL() string {
	return os.Getenv("REPO_URL")
}
