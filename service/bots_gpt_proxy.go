package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type GPTProxyPayload struct {
	Message     string    `json:"message"`
	Prompt      string    `json:"prompt"`
	Messages    []Message `json:"messages,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
}

type ChatGPTProxyBot struct {
	ID       int
	ProxyURL string
	ProxyKey string
}

func (b *ChatGPTProxyBot) GetID() int {
	return b.ID
}

func (b *ChatGPTProxyBot) Process(msg string, chatId int, api *BotAPI) {
	mid := api.Send("", chatId, b.ID)

	hs := api.GetHistory(chatId, 5)
	msgs := make([]Message, 1, 6)
	msgs[0] = Message{Content: "You are a helpful assistant", Role: "system"}

	for i := len(hs) - 1; i >= 0; i-- {
		if hs[i].ID == mid {
			continue
		}

		var role string
		text := hs[i].Text
		if len(text) > 380 {
			text = text[:380]
		}
		if hs[i].UserID == b.ID {
			role = "assistant"
		} else {
			role = "user"
		}

		msgs = append(msgs, Message{
			Content: text,
			Role:    role,
		})
	}

	rq := GPTProxyPayload{
		Messages:    msgs,
		MaxTokens:   500,
		Temperature: 0.7,
	}

	payload, err := json.Marshal(rq)
	if err != nil {
		api.Error(err, chatId, b.ID, mid)
		return
	}

	req, err := http.NewRequest("POST", b.ProxyURL, bytes.NewReader(payload))
	if err != nil {
		api.Error(err, chatId, b.ID, mid)
	}

	req.Header.Set("WX_PROXY_KEY", b.ProxyKey)
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		api.Error(err, chatId, b.ID, mid)
		return
	}

	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		api.Error(err, chatId, b.ID, mid)
		return
	}

	if res.StatusCode != 200 {
		api.Error(fmt.Errorf("code: %d, message: %s", res.StatusCode, string(body)), chatId, b.ID, mid)
		return
	}

	api.Append(mid, string(body), true, b.ID)
}
