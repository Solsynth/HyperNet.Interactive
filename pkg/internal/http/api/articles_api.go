package api

import (
	"git.solsynth.dev/hypernet/nexus/pkg/nex/cruda"
	"git.solsynth.dev/hypernet/nexus/pkg/nex/sec"
	"git.solsynth.dev/hypernet/passport/pkg/authkit"
	authm "git.solsynth.dev/hypernet/passport/pkg/authkit/models"
	"strconv"
	"time"

	"git.solsynth.dev/hypernet/interactive/pkg/internal/database"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/gap"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/http/exts"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/models"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/services"
	"github.com/gofiber/fiber/v2"
	jsoniter "github.com/json-iterator/go"
	"github.com/samber/lo"
)

func createArticle(c *fiber.Ctx) error {
	if err := sec.EnsureGrantedPerm(c, "CreatePosts", true); err != nil {
		return err
	}
	user := c.Locals("user").(authm.Account)

	var data struct {
		Publisher      uint              `json:"publisher"`
		Alias          *string           `json:"alias"`
		Title          string            `json:"title" validate:"required,max=1024"`
		Description    *string           `json:"description"`
		Content        string            `json:"content" validate:"required"`
		Thumbnail      string            `json:"thumbnail"`
		Attachments    []string          `json:"attachments"`
		Tags           []models.Tag      `json:"tags"`
		Categories     []models.Category `json:"categories"`
		PublishedAt    *time.Time        `json:"published_at"`
		PublishedUntil *time.Time        `json:"published_until"`
		VisibleUsers   []uint            `json:"visible_users_list"`
		InvisibleUsers []uint            `json:"invisible_users_list"`
		Visibility     *int8             `json:"visibility"`
		IsDraft        bool              `json:"is_draft"`
	}

	if err := exts.BindAndValidate(c, &data); err != nil {
		return err
	}

	publisher, err := services.GetPublisher(data.Publisher, user.ID)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	body := models.PostArticleBody{
		Thumbnail:   data.Thumbnail,
		Title:       data.Title,
		Description: data.Description,
		Content:     data.Content,
		Attachments: data.Attachments,
	}

	var bodyMapping map[string]any
	rawBody, _ := jsoniter.Marshal(body)
	_ = jsoniter.Unmarshal(rawBody, &bodyMapping)

	item := models.Post{
		Alias:          data.Alias,
		Type:           models.PostTypeArticle,
		Body:           bodyMapping,
		Language:       services.DetectLanguage(data.Content),
		Tags:           data.Tags,
		Categories:     data.Categories,
		IsDraft:        data.IsDraft,
		PublishedAt:    data.PublishedAt,
		PublishedUntil: data.PublishedUntil,
		VisibleUsers:   data.VisibleUsers,
		InvisibleUsers: data.InvisibleUsers,
		PublisherID:    publisher.ID,
	}

	if item.PublishedAt == nil {
		item.PublishedAt = lo.ToPtr(time.Now())
	}

	if data.Visibility != nil {
		item.Visibility = *data.Visibility
	} else {
		item.Visibility = models.PostVisibilityAll
	}

	item, err = services.NewPost(publisher, item)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	} else {
		_ = authkit.AddEventExt(
			gap.Nx,
			"posts.new",
			strconv.Itoa(int(item.ID)),
			c,
		)
	}

	return c.JSON(item)
}

func editArticle(c *fiber.Ctx) error {
	id, _ := c.ParamsInt("postId", 0)
	if err := sec.EnsureAuthenticated(c); err != nil {
		return err
	}
	user := c.Locals("user").(authm.Account)

	var data struct {
		Publisher      uint              `json:"publisher"`
		Alias          *string           `json:"alias"`
		Title          string            `json:"title" validate:"required,max=1024"`
		Description    *string           `json:"description"`
		Content        string            `json:"content" validate:"required"`
		Thumbnail      string            `json:"thumbnail"`
		Attachments    []string          `json:"attachments"`
		Tags           []models.Tag      `json:"tags"`
		Categories     []models.Category `json:"categories"`
		PublishedAt    *time.Time        `json:"published_at"`
		PublishedUntil *time.Time        `json:"published_until"`
		VisibleUsers   []uint            `json:"visible_users_list"`
		InvisibleUsers []uint            `json:"invisible_users_list"`
		Visibility     *int8             `json:"visibility"`
		IsDraft        bool              `json:"is_draft"`
	}

	if err := exts.BindAndValidate(c, &data); err != nil {
		return err
	}

	publisher, err := services.GetPublisher(data.Publisher, user.ID)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	var item models.Post
	if err := database.C.Where(models.Post{
		BaseModel:   cruda.BaseModel{ID: uint(id)},
		PublisherID: publisher.ID,
	}).First(&item).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, err.Error())
	}

	if item.LockedAt != nil {
		return fiber.NewError(fiber.StatusForbidden, "post was locked")
	}

	if !item.IsDraft && !data.IsDraft {
		item.EditedAt = lo.ToPtr(time.Now())
	}

	if item.IsDraft && !data.IsDraft && data.PublishedAt == nil {
		item.PublishedAt = lo.ToPtr(time.Now())
	} else {
		item.PublishedAt = data.PublishedAt
	}

	body := models.PostArticleBody{
		Thumbnail:   data.Thumbnail,
		Title:       data.Title,
		Description: data.Description,
		Content:     data.Content,
		Attachments: data.Attachments,
	}

	var bodyMapping map[string]any
	rawBody, _ := jsoniter.Marshal(body)
	_ = jsoniter.Unmarshal(rawBody, &bodyMapping)

	item.Alias = data.Alias
	item.Body = bodyMapping
	item.Language = services.DetectLanguage(data.Content)
	item.Tags = data.Tags
	item.Categories = data.Categories
	item.IsDraft = data.IsDraft
	item.PublishedUntil = data.PublishedUntil
	item.VisibleUsers = data.VisibleUsers
	item.InvisibleUsers = data.InvisibleUsers

	// Preload publisher data
	item.Publisher = publisher

	if data.Visibility != nil {
		item.Visibility = *data.Visibility
	}

	if item, err = services.EditPost(item); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	} else {
		_ = authkit.AddEventExt(
			gap.Nx,
			"posts.edit",
			strconv.Itoa(int(item.ID)),
			c,
		)
	}

	return c.JSON(item)
}
