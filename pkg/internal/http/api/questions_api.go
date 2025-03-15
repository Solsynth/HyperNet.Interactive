package api

import (
	"context"
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
	wproto "git.solsynth.dev/hypernet/wallet/pkg/proto"
	"github.com/gofiber/fiber/v2"
	jsoniter "github.com/json-iterator/go"
	"github.com/samber/lo"
)

func createQuestion(c *fiber.Ctx) error {
	if err := sec.EnsureGrantedPerm(c, "CreatePosts", true); err != nil {
		return err
	}
	user := c.Locals("user").(authm.Account)

	var data struct {
		Publisher      uint              `json:"publisher"`
		Alias          *string           `json:"alias"`
		Title          *string           `json:"title"`
		Content        string            `json:"content" validate:"max=4096"`
		Location       *string           `json:"location"`
		Thumbnail      *string           `json:"thumbnail"`
		Attachments    []string          `json:"attachments"`
		Tags           []models.Tag      `json:"tags"`
		Categories     []models.Category `json:"categories"`
		PublishedAt    *time.Time        `json:"published_at"`
		PublishedUntil *time.Time        `json:"published_until"`
		VisibleUsers   []uint            `json:"visible_users_list"`
		InvisibleUsers []uint            `json:"invisible_users_list"`
		Visibility     *int8             `json:"visibility"`
		IsDraft        bool              `json:"is_draft"`
		Realm          *uint             `json:"realm"`
		Reward         float64           `json:"reward"`
	}

	if err := exts.BindAndValidate(c, &data); err != nil {
		return err
	} else if len(data.Content) == 0 && len(data.Attachments) == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "content or attachments are required")
	}

	publisher, err := services.GetPublisher(data.Publisher, user.ID)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	// Take charge
	if data.Reward > 0 {
		conn, err := gap.Nx.GetClientGrpcConn("wa")
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("unable to connect Wallet: %v", err))
		}
		wc := wproto.NewPaymentServiceClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
		defer cancel()
		if _, err := wc.MakeTransactionWithAccount(ctx, &wproto.MakeTransactionWithAccountRequest{
			Amount:         data.Reward,
			Remark:         "As reward for posting a question",
			PayerAccountId: lo.ToPtr(uint64(user.ID)),
		}); err != nil {
			return fiber.NewError(fiber.StatusPaymentRequired, fmt.Sprintf("failed to handle payment: %v", err))
		}
	}

	body := models.PostQuestionBody{
		PostStoryBody: models.PostStoryBody{
			Thumbnail:   data.Thumbnail,
			Title:       data.Title,
			Content:     data.Content,
			Location:    data.Location,
			Attachments: data.Attachments,
		},
		Reward: data.Reward,
	}

	var bodyMapping map[string]any
	rawBody, _ := jsoniter.Marshal(body)
	_ = jsoniter.Unmarshal(rawBody, &bodyMapping)

	item := models.Post{
		Alias:          data.Alias,
		Type:           models.PostTypeQuestion,
		Body:           bodyMapping,
		Language:       services.DetectLanguage(data.Content),
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
		// Failed to create post, refund the charge
		if data.Reward > 0 {
			conn, err := gap.Nx.GetClientGrpcConn("wa")
			if err != nil {
				return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("unable to connect Wallet: %v", err))
			}
			wc := wproto.NewPaymentServiceClient(conn)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
			defer cancel()
			if _, err := wc.MakeTransactionWithAccount(ctx, &wproto.MakeTransactionWithAccountRequest{
				Amount:         data.Reward,
				Remark:         "As reward for posting a question - Refund",
				PayeeAccountId: lo.ToPtr(uint64(user.ID)),
			}); err != nil {
				return fiber.NewError(fiber.StatusPaymentRequired, fmt.Sprintf("failed to handle payment: %v", err))
			}
		}

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

func editQuestion(c *fiber.Ctx) error {
	id, _ := c.ParamsInt("postId", 0)
	if err := sec.EnsureAuthenticated(c); err != nil {
		return err
	}
	user := c.Locals("user").(authm.Account)

	var data struct {
		Publisher      uint              `json:"publisher"`
		Alias          *string           `json:"alias"`
		Title          *string           `json:"title"`
		Content        string            `json:"content" validate:"max=4096"`
		Thumbnail      *string           `json:"thumbnail"`
		Location       *string           `json:"location"`
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
	} else if len(data.Content) == 0 && len(data.Attachments) == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "content or attachments are required")
	}

	publisher, err := services.GetPublisher(data.Publisher, user.ID)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	var item models.Post
	if err := database.C.Where(models.Post{
		BaseModel:   cruda.BaseModel{ID: uint(id)},
		PublisherID: publisher.ID,
		Type:        models.PostTypeQuestion,
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

	var body models.PostQuestionBody
	raw, _ := jsoniter.Marshal(item.Body)
	_ = jsoniter.Unmarshal(raw, &body)

	newBody := models.PostQuestionBody{
		PostStoryBody: models.PostStoryBody{
			Thumbnail:   data.Thumbnail,
			Title:       data.Title,
			Content:     data.Content,
			Location:    data.Location,
			Attachments: data.Attachments,
		},
		Reward: body.Reward,
		Answer: body.Answer,
	}

	var newBodyMapping map[string]any
	rawBody, _ := jsoniter.Marshal(newBody)
	_ = jsoniter.Unmarshal(rawBody, &newBodyMapping)

	og := item
	item.Alias = data.Alias
	item.Body = newBodyMapping
	item.Language = services.DetectLanguage(data.Content)
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

func selectQuestionAnswer(c *fiber.Ctx) error {
	id, _ := c.ParamsInt("postId", 0)
	if err := sec.EnsureAuthenticated(c); err != nil {
		return err
	}
	user := c.Locals("user").(authm.Account)

	var data struct {
		Publisher uint `json:"publisher"`
		AnswerID  uint `json:"answer_id" validate:"required"`
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
		Type:        models.PostTypeQuestion,
	}).Preload("Publisher").First(&item).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, err.Error())
	}

	var body models.PostQuestionBody
	raw, _ := jsoniter.Marshal(item.Body)
	_ = jsoniter.Unmarshal(raw, &body)

	if item.LockedAt != nil {
		return fiber.NewError(fiber.StatusForbidden, "post was locked")
	}
	if body.Answer != nil && *body.Answer > 0 {
		return fiber.NewError(fiber.StatusBadRequest, "question already has an answer")
	}

	var answer models.Post
	if err := database.C.Where("id = ? AND reply_id = ?", data.AnswerID, item.ID).Preload("Publisher").First(&answer).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, fmt.Sprintf("related answer was not found: %v", err))
	}

	item.Body["answer"] = answer.ID

	// Preload publisher data
	item.Publisher = publisher
	if item, err = services.EditPost(item, item); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	} else {
		// Give the reward
		if body.Reward > 0 && answer.Publisher.AccountID != nil {
			conn, err := gap.Nx.GetClientGrpcConn("wa")
			if err != nil {
				return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("unable to connect Wallet: %v", err))
			}
			wc := wproto.NewPaymentServiceClient(conn)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
			defer cancel()
			if _, err := wc.MakeTransactionWithAccount(ctx, &wproto.MakeTransactionWithAccountRequest{
				Amount:         body.Reward,
				Remark:         fmt.Sprintf("Answer of question %d got selected reward", item.ID),
				PayeeAccountId: lo.ToPtr(uint64(*answer.Publisher.AccountID)),
			}); err != nil {
				return fiber.NewError(fiber.StatusPaymentRequired, fmt.Sprintf("failed to handle payment: %v", err))
			}
		}

		_ = authkit.AddEventExt(
			gap.Nx,
			"posts.edit.answer",
			map[string]any{
				"post":   item,
				"answer": answer,
			},
			c,
		)
	}

	return c.JSON(item)
}
