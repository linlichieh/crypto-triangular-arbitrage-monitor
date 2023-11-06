package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/spf13/viper"
)

type Messenger struct {
	Token          string
	SendMessageURL string
	Channel        *Channel
}

type Channel struct {
	Watch      string
	SystemLogs string
}

func initMessenger() *Messenger {
	channel := Channel{}
	if viper.GetString("ENV") == "prod" {
		channel.Watch = viper.GetString("SLACK_PROD_CHANNEL_WATCH")
		channel.SystemLogs = viper.GetString("SLACK_PROD_CHANNEL_SYSTEM_LOGS")
	} else {
		channel.Watch = viper.GetString("SLACK_DEV_CHANNEL_WATCH")
		channel.SystemLogs = viper.GetString("SLACK_DEV_CHANNEL_SYSTEM_LOGS")
	}

	return &Messenger{
		Token:          viper.GetString("SLACK_TOKEN"),
		SendMessageURL: viper.GetString("SLACK_SEND_MESSAGE_URL"),
		Channel:        &channel,
	}
}

// SlackRequestBody structure to hold the message payload
type SlackRequestBody struct {
	Channel string `json:"channel"`
	Text    string `json:"text"`
}

func (m *Messenger) sendSlackNotification(channel string, msg string) error {
	slackBody, err := json.Marshal(SlackRequestBody{Channel: channel, Text: msg})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, m.SendMessageURL, bytes.NewBuffer(slackBody))
	if err != nil {
		return err
	}

	req.Header.Add("Authorization", "Bearer "+m.Token)
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	_, err = io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return nil
}

// System log
func (m *Messenger) sendToSystemLogs(msg string) {
	log.Println(msg)
	err := m.sendSlackNotification(m.Channel.SystemLogs, msg)
	if err != nil {
		log.Printf("Error sending message to '%s': %v\n", m.Channel.SystemLogs, err)
	}
}

// Triangular arbitrage found
func (m *Messenger) sendToWatch(msg string) {
	log.Println(msg)
	err := m.sendSlackNotification(m.Channel.Watch, "<!everyone>\n"+msg)
	if err != nil {
		log.Printf("Error sending message to '%s': %v\n", m.Channel.Watch, err)
	}
}
