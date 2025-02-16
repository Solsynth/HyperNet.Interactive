package api

import (
	"fmt"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/database"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/models"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/services"
	vocab "github.com/go-ap/activitypub"
	"github.com/gofiber/fiber/v2"
	"github.com/samber/lo"
	"time"
)

func apGetPublisher(c *fiber.Ctx) error {
	name := c.Params("name")

	var publisher models.Publisher
	if err := database.C.Where("name = ?", name).First(&publisher).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, err.Error())
	}

	url := vocab.ID("https://solsynth.dev/publishers/" + publisher.Name)
	actor := vocab.Actor{
		ID:   url,
		Type: vocab.PersonType,
		Name: vocab.DefaultNaturalLanguageValue(publisher.Nick),
		URL:  url,
		Icon: vocab.Image{},
	}

	return c.JSON(actor)
}

func apGetPost(c *fiber.Ctx) error {
	take := c.QueryInt("take", 0)
	offset := c.QueryInt("offset", 0)

	tx := database.C

	var err error
	if tx, err = universalPostFilter(c, tx); err != nil {
		return err
	}

	items, err := services.ListPost(tx, take, offset, "published_at DESC")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	if c.QueryBool("truncate", true) {
		for _, item := range items {
			if item != nil {
				item = lo.ToPtr(services.TruncatePostContent(*item))
			}
		}
	}

	var acts []vocab.Activity
	for _, item := range items {
		pubUrl := vocab.ID("https://solsynth.dev/publishers/" + item.Publisher.Name)
		url := fmt.Sprintf("https://solsynth.dev/posts/%d", item.ID)
		content, ok := item.Body["content"].(string)
		if !ok {
			content = "Posted a post"
		}
		acts = append(acts, vocab.Activity{
			ID:   vocab.ID(url),
			Type: vocab.CreateType,
			Actor: vocab.Actor{
				ID:   pubUrl,
				Type: vocab.PersonType,
				Name: vocab.DefaultNaturalLanguageValue(item.Publisher.Nick),
				URL:  pubUrl,
				Icon: vocab.Image{},
			},
			Object: vocab.Object{
				ID:   vocab.ID(url),
				Type: vocab.NoteType,
				Name: vocab.DefaultNaturalLanguageValue(content),
				URL:  vocab.ID(url),
				Icon: vocab.Image{},
			},
			Published: lo.TernaryF(item.PublishedAt != nil, func() time.Time {
				return *item.PublishedAt
			}, func() time.Time {
				return item.CreatedAt
			}),
		})
	}

	return c.JSON(acts)
}
