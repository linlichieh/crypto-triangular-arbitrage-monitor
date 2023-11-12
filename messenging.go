package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/spf13/viper"
)

const (
	SEND_TO_SYSTEM_LOGS_INTERVAL_SECOND = 3
	SLACK_CHANNEL_WATCH                 = "watch"
	SLACK_CHANNEL_SYSTEM_LOGS           = "system_logs"
)

type Messenger struct {
	Token          string
	SendMessageURL string
	ChannelMap     map[string]*Channel
}

type Channel struct {
	Name string
	Chan chan string
}

// SlackRequestBody structure to hold the message payload
type SlackRequestBody struct {
	Channel string `json:"channel"`
	Text    string `json:"text"`
}

func initMessenger() *Messenger {
	return &Messenger{
		Token:          viper.GetString("SLACK_TOKEN"),
		SendMessageURL: viper.GetString("SLACK_SEND_MESSAGE_URL"),
		ChannelMap:     loadChannelMap(),
	}
}

func loadChannelMap() map[string]*Channel {
	channelMap := make(map[string]*Channel)
	channelWatch := Channel{
		Name: viper.GetString("SLACK_CHANNEL_WATCH"),
		Chan: make(chan string),
	}
	channelSystemLogs := Channel{
		Name: viper.GetString("SLACK_CHANNEL_SYSTEM_LOGS"),
		Chan: make(chan string),
	}
	channelMap[SLACK_CHANNEL_WATCH] = &channelWatch
	channelMap[SLACK_CHANNEL_SYSTEM_LOGS] = &channelSystemLogs

	return channelMap
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

func (m *Messenger) SystemLogs(msg string) {
	m.ChannelMap[SLACK_CHANNEL_SYSTEM_LOGS].Chan <- msg
}

func (m *Messenger) sendToChannel(channel string, msg string) {
	log.Println(msg)
	err := m.sendSlackNotification(channel, msg)
	if err != nil {
		log.Printf("Error sending message to '%s': %v\n", channel, err)
	}
}

func (m *Messenger) handleChannelSystemLogs() {
	ticker := time.NewTicker(time.Duration(SEND_TO_SYSTEM_LOGS_INTERVAL_SECOND) * time.Second)
	defer ticker.Stop()

	// To show counters for result e.g. `map[997:1762 998:466]` means result 997 gets 1762 times, 998 gets 466 times
	var combinedMsg string
	for {
		select {
		case msg := <-m.ChannelMap[SLACK_CHANNEL_SYSTEM_LOGS].Chan:
			combinedMsg += fmt.Sprintf("%s\n", msg)
		case <-ticker.C:
			if combinedMsg == "" {
				continue
			}
			go m.sendToChannel(m.ChannelMap[SLACK_CHANNEL_SYSTEM_LOGS].Name, combinedMsg)

			// Reset the combined message
			combinedMsg = ""
		}
	}
}
