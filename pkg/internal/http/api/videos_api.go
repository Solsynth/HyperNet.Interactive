package api

import (
	"fmt"
	"time"

	"git.solsynth.dev/hypernet/interactive/pkg/internal/database"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/gap"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/http/exts"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/models"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/services"
	"git.solsynth.dev/hypernet/nexus/pkg/nex/cruda"
	"git.solsynth.dev/hypernet/nexus/pkg/nex/sec"
	"git.solsynth.dev/hypernet/passport/pkg/authkit"
	authm "git.solsynth.dev/hypernet/passport/pkg/authkit/models"
	"github.com/gofiber/fiber/v2"
	jsoniter "github.com/json-iterator/go"
	"github.com/samber/lo"
)

func createVideo(c *fiber.Ctx) error {
	if err := sec.EnsureGrantedPerm(c, "CreatePosts", true); err != nil {
		return err
	}
	user := c.Locals("user").(authm.Account)

	var data struct {
		Publisher      uint              `json:"publisher"`
		Video          string            `json:"video" validate:"required"`
		Alias          *string           `json:"alias"`
		Title          string            `json:"title" validate:"required"`
		Description    *string           `json:"description"`
		Location       *string           `json:"location"`
		Thumbnail      *string           `json:"thumbnail"`
		Subtitles      map[string]string `json:"subtitles"`
		Tags           []models.Tag      `json:"tags"`
		Categories     []models.Category `json:"categories"`
		PublishedAt    *time.Time        `json:"published_at"`
		PublishedUntil *time.Time        `json:"published_until"`
		VisibleUsers   []uint            `json:"visible_users_list"`
		InvisibleUsers []uint            `json:"invisible_users_list"`
		Visibility     *int8             `json:"visibility"`
		IsDraft        bool              `json:"is_draft"`
		Realm          *uint             `json:"realm"`
	}

	if err := exts.BindAndValidate(c, &data); err != nil {
		return err
	}

	publisher, err := services.GetPublisher(data.Publisher, user.ID)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	body := models.PostVideoBody{
		Thumbnail:   data.Thumbnail,
		Video:       data.Video,
		Title:       data.Title,
		Description: data.Description,
		Location:    data.Location,
		Subtitles:   data.Subtitles,
	}

	var bodyMapping map[string]any
	rawBody, _ := jsoniter.Marshal(body)
	_ = jsoniter.Unmarshal(rawBody, &bodyMapping)

	item := models.Post{
		Alias:          data.Alias,
		Type:           models.PostTypeVideo,
		Body:           bodyMapping,
		Language:       services.DetectLanguage(data.Title),
		Tags:           data.Tags,
		Categories:     data.Categories,
		PublishedAt:    data.PublishedAt,
		PublishedUntil: data.PublishedUntil,
		IsDraft:        data.IsDraft,
		VisibleUsers:   data.VisibleUsers,
		InvisibleUsers: data.InvisibleUsers,
		PublisherID:    publisher.ID,
	}

	if item.PublishedAt == nil {
		item.PublishedAt = lo.ToPtr(time.Now())
	}

	if data.Realm != nil {
		if _, err := authkit.GetRealmMember(gap.Nx, *data.Realm, user.ID); err != nil {
			return fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf("you are not a member of realm #%d", *data.Realm))
		}
		item.RealmID = data.Realm
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
			map[string]any{"post": item},
			c,
		)
	}

	return c.JSON(item)
}

func editVideo(c *fiber.Ctx) error {
	id, _ := c.ParamsInt("postId", 0)
	if err := sec.EnsureAuthenticated(c); err != nil {
		return err
	}
	user := c.Locals("user").(authm.Account)

	var data struct {
		Publisher      uint              `json:"publisher"`
		Video          string            `json:"video" validate:"required"`
		Alias          *string           `json:"alias"`
		Title          string            `json:"title" validate:"required"`
		Description    *string           `json:"description"`
		Location       *string           `json:"location"`
		Thumbnail      *string           `json:"thumbnail"`
		Subtitles      map[string]string `json:"subtitles"`
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
		Type:        models.PostTypeVideo,
	}).Preload("Publisher").First(&item).Error; err != nil {
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

	body := models.PostVideoBody{
		Thumbnail:   data.Thumbnail,
		Video:       data.Video,
		Title:       data.Title,
		Description: data.Description,
		Location:    data.Location,
		Subtitles:   data.Subtitles,
	}

	var bodyMapping map[string]any
	rawBody, _ := jsoniter.Marshal(body)
	_ = jsoniter.Unmarshal(rawBody, &bodyMapping)

	og := item
	item.Alias = data.Alias
	item.Body = bodyMapping
	item.Language = services.DetectLanguage(data.Title)
	item.Tags = data.Tags
	item.Categories = data.Categories
	item.IsDraft = data.IsDraft
	item.PublishedUntil = data.PublishedUntil
	item.VisibleUsers = data.VisibleUsers
	item.InvisibleUsers = data.InvisibleUsers

	// Preload publisher data
	item.Publisher = publisher

	if item.PublishedAt == nil {
		item.PublishedAt = data.PublishedAt
	}
	if data.Visibility != nil {
		item.Visibility = *data.Visibility
	}

	if item, err = services.EditPost(item, og); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	} else {
		_ = authkit.AddEventExt(
			gap.Nx,
			"posts.edit",
			map[string]any{"post": item},
			c,
		)
	}

	return c.JSON(item)
}
