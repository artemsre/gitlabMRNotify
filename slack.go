package main

import (
	"fmt"
	"github.com/slack-go/slack"
	"github.com/worr/secstring"
	"os"
)

func sendSlackMessage(message string) string {
	id := os.Getenv("SLACK-CHANNEL")
	token := os.Getenv("SLACK-TOKEN")
	ss, _ := secstring.FromString(&token)
	defer ss.Destroy()
	api := slack.New(string(ss.String))
	channelID, timestamp, err := api.PostMessage(id, slack.MsgOptionText(message, false))
	if err != nil {
		fmt.Printf("%s\n", err)
		return ""
	}
	fmt.Printf("Message successfully sent to channel %s at %s", channelID, timestamp)
	return fmt.Sprintf("%s", timestamp)
}
func editSlackMessage(tstamp string, message string) {
	id := os.Getenv("SLACK-CHANNEL")
	token := os.Getenv("SLACK-TOKEN")
	ss, _ := secstring.FromString(&token)
	defer ss.Destroy()
	api := slack.New(string(ss.String))
	api.UpdateMessage(id, tstamp, slack.MsgOptionText(message, false))
}
