package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

var client = &http.Client{Timeout: 10 * time.Second}

type sendMessageRequest struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode"`
}

// Send posts a message to the configured Telegram group.
// It is a no-op if token or chatID are empty.
func Send(token, chatID, text string) error {
	if token == "" || chatID == "" {
		return nil
	}

	body, err := json.Marshal(sendMessageRequest{
		ChatID:    chatID,
		Text:      text,
		ParseMode: "HTML",
	})
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram: unexpected status %d", resp.StatusCode)
	}
	return nil
}
