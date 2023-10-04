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

	hs := api.GetHistory(chatId, 3)
	msgs := make([]Message, len(hs)+2)
	msgs[0] = Message{Content: "You are a helpful assistant", Role: "system"}
	for i, x := range hs {
		var role string
		text := x.Text
		if len(text) > 500 {
			text = text[:500]
		}
		if x.UserID == b.ID {
			role = "assistant"
		} else {
			role = "user"
		}

		msgs[i+1] = Message{
			Content: text,
			Role:    role,
		}
	}
	msgs[len(hs)+1] = Message{Content: msg, Role: "user"}
	fmt.Printf("History: %+v\n", msgs)

	rq := GPTProxyPayload{
		Messages:    msgs,
		MaxTokens:   500,
		Temperature: 0.7,
	}

	payload, err := json.Marshal(rq)
	if err != nil {
		api.Error(err, chatId, b.ID)
		return
	}

	req, err := http.NewRequest("POST", b.ProxyURL, bytes.NewReader(payload))
	if err != nil {
		api.Error(err, chatId, b.ID)
	}

	req.Header.Set("WX_PROXY_KEY", b.ProxyKey)
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		api.Error(err, chatId, b.ID)
		return
	}

	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		api.Error(err, chatId, b.ID)
		return
	}

	if res.StatusCode != 200 {
		api.Error(fmt.Errorf("code: %d, message: %s", res.StatusCode, string(body)), chatId, b.ID)
		return
	}

	api.Append(mid, string(body), true, b.ID)
}
