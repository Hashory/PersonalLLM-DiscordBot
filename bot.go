package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

// BotService handles bot configuration and Discord session management.
type BotService struct {
	Session *discordgo.Session
	Config  *Config
}

// NewBotService creates a new instance of BotService.
func NewBotService(session *discordgo.Session, config *Config) *BotService {
	return &BotService{
		Session: session,
		Config:  config,
	}
}

// Open creates a websocket connection to Discord
func (bs *BotService) Open() error {
	if err := bs.Session.Open(); err != nil {
		log.Printf("Error opening connection to Discord: %v", err)
		return err
	}
	return nil
}

// Close closes the websocket connection to Discord
func (bs *BotService) Close() error {
	if err := bs.Session.Close(); err != nil {
		log.Printf("Error closing connection to Discord: %v", err)
		return err
	}
	return nil

}

// RegisterEventHandlers registers handlers for various Discord events.
func (bs *BotService) RegisterEventHandlers() {
	bs.Session.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		s.UpdateGameStatus(0, "Chat with AI")
	})

	bs.Session.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		bs.onMessageCreate(s, m)
	})
}

// onMessageCreate processes incoming messages and decides the action to take.
func (bs *BotService) onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) error {
	if m.Author.ID == s.State.User.ID || m.Type != discordgo.MessageTypeDefault {
		return nil
	}

	channelConfig := bs.Config.FindChannelConfig(m.ChannelID)
	if channelConfig != nil {
		bs.processMessage(m, channelConfig)
	} else {
		bs.checkAndReplyToThread(m)
	}

	return nil
}

// processMessage handles messages based on channel configuration.
func (bs *BotService) processMessage(m *discordgo.MessageCreate, channelConfig *ApiChannelConfig) {
	// TODO Message structure should be defined in the other file.
	apiMessage := NewApiMessage("user", m.Content)

	var threadName string
	if len(m.Content) < 50 {
		threadName = m.Content
	} else {
		threadName = m.Content[:50]
	}

	if err := bs.makeReplyWithThread(threadName, m.ChannelID, m.ID, channelConfig, []*ApiMessage{apiMessage}); err != nil {
		log.Printf("Error processing message: %v, m.Content: %s, m.ChannelID: %s, m.ID: %s", err, m.Content, m.ChannelID, m.ID)
	}
}

// makeReplyWithThread initiates a message thread and sends a reply based on API response.
func (bs *BotService) makeReplyWithThread(threadName, threadStartChannelId, threadTargetMessageId string, apiConfig *ApiChannelConfig, apiMessages []*ApiMessage) error {
	thread, err := bs.Session.MessageThreadStart(threadStartChannelId, threadTargetMessageId, threadName, 60)
	if err != nil {
		log.Printf("Failed to start message thread: %v, threadStartChannelId: %s, threadTargetMessageId: %s, threadName: %s", err, threadStartChannelId, threadTargetMessageId, threadName)
		return err
	}

	return bs.makeReplyMessage(thread.ID, apiConfig, apiMessages)
}

// checkAndReplyToThread checks for thread messages and replies accordingly.
func (bs *BotService) checkAndReplyToThread(m *discordgo.MessageCreate) {
	channel, err := bs.Session.Channel(m.ChannelID)
	if err != nil || !channel.IsThread() {
		return // Not a thread, or an error occurred
	}

	channelConfig := bs.Config.FindChannelConfig(channel.ParentID)
	if channelConfig == nil {
		return // No configuration found for this channel
	}

	threadContent, err := bs.fetchThreadContent(channel, channel.ParentID)
	if len(threadContent) == 0 || err != nil {
		return // No content to reply to
	}

	if err := bs.makeReplyMessage(m.ChannelID, channelConfig, threadContent); err != nil {
		fmt.Printf("Error replying to thread: %v\n", err)
	}
}

// fetchThreadContent retrieves messages from a thread for processing
func (bs *BotService) fetchThreadContent(channel *discordgo.Channel, parentChannelId string) ([]*ApiMessage, error) {
	threadContent, err := bs.Session.ChannelMessages(channel.ID, 100, "", "", "")
	if err != nil {
		log.Printf("Error fetching thread content: %v, channelID: %s", err, channel.ID)
		return nil, fmt.Errorf("error fetching thread content: %w", err)
	}

	var messages []*ApiMessage
	for _, tc := range threadContent {
		if tc.Type == discordgo.MessageTypeDefault {
			var role string
			if tc.Author.ID == bs.Session.State.User.ID {
				role = "assistant"
			} else {
				role = "user"
			}

			messages = append([]*ApiMessage{NewApiMessage(role, tc.Content)}, messages...)
		}
	}

	threadParentChannel, err := bs.Session.Channel(parentChannelId)
	if err != nil {
		log.Printf("Error fetching thread parent: %v, channelID: %s, parentChannelId: %s", err, channel.ParentID, parentChannelId)
		return nil, fmt.Errorf("error fetching thread parent: %w", err)
	}
	if parentChannelMessages, err := bs.Session.ChannelMessages(threadParentChannel.ID, 15, "", "", threadParentChannel.LastMessageID); err != nil {
		log.Printf("Error fetching parent channel messages: %v, channelID: %s", err, threadParentChannel.ID)
	} else {
		for _, pcm := range parentChannelMessages {
			if pcm.Thread.ID == channel.ID {
				messages = append([]*ApiMessage{NewApiMessage("user", pcm.Content)}, messages...)
				break
			}
		}
	}

	return messages, nil
}

// makeReplyMessage sends a temporary message, calls an external API, and updates the message with the API's response.
func (bs *BotService) makeReplyMessage(channelID string, channelConfig *ApiChannelConfig, messages []*ApiMessage) error {
	tempMessage, err := bs.Session.ChannelMessageSend(channelID, "Processing your request...")
	if err != nil {
		log.Printf("Error sending temporary message: %v, channelID: %s", err, channelID)
		return fmt.Errorf("error sending temporary message: %w", err)
	}

	for _, srm := range channelConfig.SystemRoleMessages {
		messages = append([]*ApiMessage{NewApiMessage("system", srm)}, messages...)
	}

	requestBody, err := NewApiRequest(channelConfig.ModelName, messages).toJSON()
	if err != nil {
		log.Printf("Error converting request to JSON: %v, Request Body: %s", err, requestBody)
		return fmt.Errorf("error failed convert to json: %w", err)
	}

	return bs.updateMessageWithAPIResponse(channelID, tempMessage.ID, channelConfig.ApiUrl, channelConfig.ApiAuthToken, requestBody)
}

// updateMessageWithAPIResponse handles sending a request to the external API and updating the Discord message with the response.
func (bs *BotService) updateMessageWithAPIResponse(channelID, messageID, apiUrl, apiAuthToken string, requestBody []byte) error {
	req, err := http.NewRequest("POST", apiUrl, bytes.NewBuffer(requestBody))
	if err != nil {
		log.Printf("Error creating HTTP request for API: %v, apiUrl: %s , requestBody: %s", err, apiUrl, requestBody)
		return fmt.Errorf("error creating HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if apiAuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+apiAuthToken)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error sending request to API: %v, apiUrl: %s , requestBody: %s", err, apiUrl, requestBody)
		bs.Session.ChannelMessageEdit(channelID, messageID, "😵 An error occurred in the bot")
		return fmt.Errorf("error sending request to API: %w", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	var allContent strings.Builder

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if _, err := bs.Session.ChannelMessageEdit(channelID, messageID, allContent.String()); err != nil {
				log.Printf("Error updating message: %v, channelID: %s, messageID: %s, allContent: %s", err, channelID, messageID, allContent.String())
				fmt.Printf("Error updating message: %v\n", err)
				return err
			}
		default:
			line, err := reader.ReadBytes('\n')
			if err != nil {
				log.Printf("Stream closed or error occurred: %v, line: %s", err, line)
				fmt.Printf("Stream closed or error occurred: %v\n", err)
				bs.Session.ChannelMessageEdit(channelID, messageID, "😵 An error occurred in the bot")
				return err
			}

			respContent, err := ApiResponseFromJSON(line)
			if err != nil {
				log.Printf("Error decoding JSON: %v, line: %s", err, line)
				fmt.Printf("Error decoding JSON: %v\n", err)
				continue // Skip malformed JSON lines
			}

			// Append new content to the allContent string builder
			allContent.WriteString(respContent.Message.Content)

			// If 'done' flag is received, finalize the message edit and exit
			if respContent.Done {
				if _, err := bs.Session.ChannelMessageEdit(channelID, messageID, allContent.String()); err != nil {
					log.Printf("Error finalizing message update: %v, channelID: %s, messageID: %s, allContent: %s", err, channelID, messageID, allContent.String())
					fmt.Printf("Error finalizing message update: %v\n", err)
				}
				return nil
			}
		}
	}
}
