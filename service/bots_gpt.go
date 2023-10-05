package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type GPTPayload struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
}

type GPTResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Choices []Choice `json:"choices"`
}

type Choice struct {
	Message      ChoiceMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

type ChoiceMessage struct {
	Content string `json:"content"`
}

type GTPErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    int    `json:"int"`
	} `json:"error"`
}

type ChatGPTBot struct {
	ID  int
	Key string
}

func (b *ChatGPTBot) GetID() int {
	return b.ID
}

func (b *ChatGPTBot) Process(msg string, chatId int, api *BotAPI) {
	rq := GPTPayload{
		Model: "gpt-3.5-turbo",
		Messages: []Message{
			{
				Role:    "system",
				Content: "You are a helpful assistant",
			},
			{
				Role:    "user",
				Content: msg,
			},
		},
		MaxTokens:   500,
		Temperature: 0.7,
	}

	payload, err := json.Marshal(rq)
	if err != nil {
		api.Error(err, chatId, b.ID, 0)
		return
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		api.Error(err, chatId, b.ID, 0)
	}

	req.Header.Set("Authorization", "Bearer "+b.Key)
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		api.Error(err, chatId, b.ID, 0)
		return
	}

	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		api.Error(err, chatId, b.ID, 0)
		return
	}

	if res.StatusCode != 200 {
		errRes := GTPErrorResponse{}
		err := json.Unmarshal(body, &errRes)
		if err != nil {
			api.Error(err, chatId, b.ID, 0)
			return
		}

		api.Error(fmt.Errorf("code: %d, message: %s", errRes.Error.Code, errRes.Error.Message), chatId, b.ID, 0)
		return
	}

	resp := GPTResponse{}
	err = json.Unmarshal(body, &resp)

	if err != nil {
		api.Error(err, chatId, b.ID, 0)
		return
	}

	if len(resp.Choices) == 0 {
		api.Error(errors.New("response doesn't have any choices"), chatId, b.ID, 0)
		return
	}

	api.Send(resp.Choices[0].Message.Content, chatId, b.ID)
}
