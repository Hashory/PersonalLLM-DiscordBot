package main

import (
	"encoding/json"
	"log"
)

// ApiMessage represents the structure for sending instructions to a Chat type LLM.
// This structure is part of the ApiRequest.
type ApiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// NewApiMessage creates and returns a new ApiMessage instance.
func NewApiMessage(role, content string) *ApiMessage {
	return &ApiMessage{
		Role:    role,
		Content: content,
	}
}

// ApiRequest represents the request to the external API.
type ApiRequest struct {
	ModelName string       `json:"model"`
	Stream    bool         `json:"stream"`
	Messages  []ApiMessage `json:"messages"`
}

// NewApiRequest creates and returns a new ApiRequest instance.
func NewApiRequest(modelName string, messages []*ApiMessage) *ApiRequest {
	// Convert []*ApiMessage to []ApiMessage
	apiMessages := make([]ApiMessage, len(messages))
	for i, msg := range messages {
		apiMessages[i] = *msg
	}

	return &ApiRequest{
		ModelName: modelName,
		Stream:    true,
		Messages:  apiMessages,
	}
}

// toJSON returns the JSON encoding of the ApiRequest instance.
func (r *ApiRequest) toJSON() ([]byte, error) {
	return json.Marshal(r)
}

// ApiResponse represents the response from the external API.
type ApiResponse struct {
	Done    bool `json:"done"`
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
}

// ApiResponseFromJSON creates and returns a new ApiResponse instance from the JSON data.
func ApiResponseFromJSON(data []byte) (*ApiResponse, error) {
	var apiResponse ApiResponse
	if err := json.Unmarshal(data, &apiResponse); err != nil {
		log.Printf("Error unmarshal API response: %v", err)
		return nil, err
	}

	return &apiResponse, nil
}
