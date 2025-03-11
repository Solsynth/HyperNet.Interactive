package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type MisskeyPost struct {
	ID   string `json:"id"`
	Text string `json:"text"`
	User struct {
		Username    string `json:"username"`
		DisplayName string `json:"name"`
		AvatarURL   string `json:"avatarUrl"`
	} `json:"user"`
	CreatedAt string `json:"createdAt"`
}

func FetchTimeline(server, token string, limit int) ([]MisskeyPost, error) {
	url := fmt.Sprintf("%s/api/notes/global-timeline", server)

	payload := map[string]interface{}{
		"limit": limit,
		"i":     token,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Misskey posts: %v", err)
	}
	defer resp.Body.Close()

	var posts []MisskeyPost
	if err := json.NewDecoder(resp.Body).Decode(&posts); err != nil {
		return nil, fmt.Errorf("failed to parse Misskey JSON: %v", err)
	}

	return posts, nil
}
