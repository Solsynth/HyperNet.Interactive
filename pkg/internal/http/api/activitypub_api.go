package api

import (
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"time"

	"git.solsynth.dev/hypernet/interactive/pkg/internal/database"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/models"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/services"
	"github.com/go-ap/activitypub"
	"github.com/gofiber/fiber/v2"
	"github.com/samber/lo"
)

func apUserInbox(c *fiber.Ctx) error {
	name := c.Params("name")

	var activity activitypub.Activity
	if err := c.BodyParser(&activity); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid activitypub event")
	}

	// TODO Handle all these
	switch activity.Type {
	case activitypub.LikeType:
		log.Printf("User %s received a Like on: %s", name, activity.Object.GetID())
	case activitypub.FollowType:
		log.Printf("User %s received a Follow request from: %s", name, activity.Actor.GetID())
	case activitypub.CreateType:
		log.Printf("New post received for %s: %s", name, activity.Object.GetID())
	default:
		log.Printf("Unhandled activity type received: %+v", activity)
	}

	return c.Status(http.StatusAccepted).SendString("Activity received")
}

func apUserOutbox(c *fiber.Ctx) error {
	name := c.Params("name")
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 10)

	if limit > 100 {
		limit = 100
	}

	var publisher models.Publisher
	if err := database.C.Where("name = ?", name).First(&publisher).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, err.Error())
	}

	tx, err := services.UniversalPostFilter(c, database.C)
	tx.Where("publisher_id = ? AND reply_id IS NULL", publisher.ID)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	count, err := services.CountPost(tx)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	var activities []activitypub.Item
	if posts, err := services.ListPostV1(tx, limit, (page-1)*limit, "published_at DESC", nil); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	} else {
		for _, post := range posts {
			var content string
			if val, ok := post.Body["content"].(string); ok {
				content = val
			} else {
				content = "Posted a post"
			}
			note := activitypub.Note{
				ID:           services.GetActivityID("/posts/" + strconv.Itoa(int(post.ID))),
				Type:         activitypub.NoteType,
				Attachment:   nil,
				AttributedTo: services.GetActivityIRI("/users/" + publisher.Name),
				Published: lo.TernaryF(post.PublishedAt == nil, func() time.Time {
					return post.CreatedAt
				}, func() time.Time {
					return *post.PublishedAt
				}),
				Updated: lo.TernaryF(post.EditedAt == nil, func() time.Time {
					return post.UpdatedAt
				}, func() time.Time {
					return *post.EditedAt
				}),
				To:      activitypub.ItemCollection{activitypub.PublicNS},
				Content: activitypub.DefaultNaturalLanguageValue(content),
			}
			activity := activitypub.Create{
				ID:     services.GetActivityID("/activities/posts/" + strconv.Itoa(int(post.ID))),
				Type:   activitypub.CreateType,
				Actor:  services.GetActivityIRI("/users/" + publisher.Name),
				Object: note,
			}
			activities = append(activities, activity)
		}
	}

	totalPages := int(math.Ceil(float64(count) / float64(limit)))

	outbox := activitypub.OrderedCollectionPage{
		ID:           services.GetActivityID("/users/" + publisher.Name + "/outbox"),
		Type:         activitypub.OrderedCollectionType,
		TotalItems:   uint(count),
		OrderedItems: activitypub.ItemCollection(activities),
		First:        services.GetActivityIRI(fmt.Sprintf("/users/%s/outbox?page=%d", publisher.Name, 1)),
		Last:         services.GetActivityIRI(fmt.Sprintf("/users/%s/outbox?page=%d", publisher.Name, totalPages)),
	}

	if page > 1 {
		outbox.Prev = services.GetActivityIRI(fmt.Sprintf("/users/%s/outbox?page=%d&limit=%d", publisher.Name, page-1, limit))
	}
	if page < totalPages {
		outbox.Next = services.GetActivityIRI(fmt.Sprintf("/users/%s/outbox?page=%d&limit=%d", publisher.Name, page+1, limit))
	}

	return c.JSON(outbox)
}

func apUserActor(c *fiber.Ctx) error {
	name := c.Params("name")

	var publisher models.Publisher
	if err := database.C.Where("name = ?", name).First(&publisher).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "User not found"})
	}

	id := services.GetActivityID("/users/" + publisher.Name)
	actor := activitypub.Actor{
		ID:                id,
		Inbox:             id + "/inbox",
		Outbox:            id + "/outbox",
		Type:              activitypub.PersonType,
		Name:              activitypub.DefaultNaturalLanguageValue(publisher.Name),
		PreferredUsername: activitypub.DefaultNaturalLanguageValue(publisher.Nick),
	}

	return c.JSON(actor)
}
