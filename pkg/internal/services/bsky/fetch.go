package bsky

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type BlueskyPost struct {
	URI    string `json:"uri"`
	CID    string `json:"cid"`
	Record struct {
		Text      string `json:"text"`
		CreatedAt string `json:"createdAt"`
	} `json:"record"`
	Author struct {
		Handle      string `json:"handle"`
		DisplayName string `json:"displayName"`
	} `json:"author"`
}

func FetchBlueskyPublicFeed(feedURI string, limit int) ([]BlueskyPost, error) {
	url := fmt.Sprintf("https://public.api.bsky.app/xrpc/app.bsky.feed.getFeed?feed=%s&limit=%d", feedURI, limit)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Bluesky posts: %v", err)
	}
	defer resp.Body.Close()

	var response struct {
		Feed []BlueskyPost `json:"feed"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to parse Bluesky JSON: %v", err)
	}

	return response.Feed, nil
}
