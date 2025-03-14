package mastodon

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"git.solsynth.dev/hypernet/interactive/pkg/internal/models"
	"git.solsynth.dev/hypernet/nexus/pkg/nex/cruda"
	"github.com/gofiber/fiber/v2"
	jsoniter "github.com/json-iterator/go"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
)

type MastodomAttachment struct {
	URL     string `json:"url"`
	Preview string `json:"preview"`
}

type MastodonPost struct {
	ID        string `json:"id"`
	Content   string `json:"content"`
	URL       string `json:"url"`
	Sensitive bool   `json:"sensitive"`
	Account   struct {
		Acct        string `json:"acct"`
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
		Avatar      string `json:"avatar"`
		AccountURL  string `json:"url"`
	} `json:"account"`
	Language         string               `json:"language"`
	MediaAttachments []MastodomAttachment `json:"media_attachments"`
	CreatedAt        time.Time            `json:"created_at"`
	Server           string               `json:"-"`
}

func (v MastodonPost) ToFediversePost() models.FediversePost {
	return models.FediversePost{
		BaseModel: cruda.BaseModel{
			CreatedAt: v.CreatedAt,
		},
		Identifier: fmt.Sprintf("%s@%s", v.ID, v.Server),
		Origin:     v.Server,
		Content:    v.Content,
		Language:   v.Language,
		Images: lo.Map(v.MediaAttachments, func(item MastodomAttachment, _ int) string {
			return item.URL
		}), User: models.FediverseUser{
			Identifier: v.Account.Acct,
			Name:       v.Account.Username,
			Nick:       v.Account.DisplayName,
			Avatar:     v.Account.Avatar,
			Origin:     v.Server,
		},
	}
}

func FetchTimeline(server string, limit int, useTrend bool) ([]MastodonPost, error) {
	url := fmt.Sprintf(
		lo.Ternary(
			useTrend,
			"%s/api/v1/trends/statuses?limit=%d",
			"%s/api/v1/timelines/public?limit=%d",
		),
		server,
		limit,
	)
	log.Debug().Str("url", url).Msg("Fetching mastodon timeline...")

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch public timeline: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, response: %s", resp.StatusCode, body)
	}

	log.Debug().Str("url", url).Msg("Fetched mastodon timeline...")

	var posts []MastodonPost
	if err := jsoniter.Unmarshal(body, &posts); err != nil {
		return nil, fmt.Errorf("failed to parse timeline JSON: %v", err)
	}

	posts = lo.Filter(posts, func(item MastodonPost, index int) bool {
		return !item.Sensitive
	})

	for idx := range posts {
		posts[idx].Server = strings.Replace(strings.Replace(server, "https://", "", 1), "http://", "", 1)
	}

	return posts, nil
}
