package services

import (
	"errors"
	"fmt"

	"git.solsynth.dev/hypernet/interactive/pkg/internal/database"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/gap"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/models"
	"git.solsynth.dev/hypernet/nexus/pkg/proto"
	"git.solsynth.dev/hypernet/passport/pkg/authkit"
	authm "git.solsynth.dev/hypernet/passport/pkg/authkit/models"
	"git.solsynth.dev/hypernet/pusher/pkg/pushkit"
	"github.com/samber/lo"
	"gorm.io/gorm"
)

func GetSubscriptionOnUser(user authm.Account, target models.Publisher) (*models.Subscription, error) {
	var subscription models.Subscription
	if err := database.C.Where("follower_id = ? AND account_id = ?", user.ID, target.ID).First(&subscription).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("unable to get subscription: %v", err)
	}
	return &subscription, nil
}

func GetSubscriptionOnTag(user authm.Account, target models.Tag) (*models.Subscription, error) {
	var subscription models.Subscription
	if err := database.C.Where("follower_id = ? AND tag_id = ?", user.ID, target.ID).First(&subscription).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("unable to get subscription: %v", err)
	}
	return &subscription, nil
}

func GetSubscriptionOnCategory(user authm.Account, target models.Category) (*models.Subscription, error) {
	var subscription models.Subscription
	if err := database.C.Where("follower_id = ? AND category_id = ?", user.ID, target.ID).First(&subscription).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("unable to get subscription: %v", err)
	}
	return &subscription, nil
}

func SubscribeToUser(user authm.Account, target models.Publisher) (models.Subscription, error) {
	var subscription models.Subscription
	if err := database.C.Where("follower_id = ? AND account_id = ?", user.ID, target.ID).First(&subscription).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return subscription, fmt.Errorf("subscription already exists")
		}
	}

	subscription = models.Subscription{
		FollowerID: user.ID,
		AccountID:  &target.ID,
	}

	err := database.C.Save(&subscription).Error
	return subscription, err
}

func SubscribeToTag(user authm.Account, target models.Tag) (models.Subscription, error) {
	var subscription models.Subscription
	if err := database.C.Where("follower_id = ? AND tag_id = ?", user.ID, target.ID).First(&subscription).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return subscription, fmt.Errorf("subscription already exists")
		}
	}

	subscription = models.Subscription{
		FollowerID: user.ID,
		TagID:      &target.ID,
	}

	err := database.C.Save(&subscription).Error
	return subscription, err
}

func SubscribeToCategory(user authm.Account, target models.Category) (models.Subscription, error) {
	var subscription models.Subscription
	if err := database.C.Where("follower_id = ? AND category_id = ?", user.ID, target.ID).First(&subscription).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return subscription, fmt.Errorf("subscription already exists")
		}
	}

	subscription = models.Subscription{
		FollowerID: user.ID,
		CategoryID: &target.ID,
	}

	err := database.C.Save(&subscription).Error
	return subscription, err
}

func UnsubscribeFromUser(user authm.Account, target models.Publisher) error {
	var subscription models.Subscription
	if err := database.C.Where("follower_id = ? AND account_id = ?", user.ID, target.ID).First(&subscription).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("subscription does not exist")
		}
		return fmt.Errorf("unable to check subscription is exists or not: %v", err)
	}

	err := database.C.Delete(&subscription).Error
	return err
}

func UnsubscribeFromTag(user authm.Account, target models.Tag) error {
	var subscription models.Subscription
	if err := database.C.Where("follower_id = ? AND tag_id = ?", user.ID, target.ID).First(&subscription).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("subscription does not exist")
		}
		return fmt.Errorf("unable to check subscription is exists or not: %v", err)
	}

	err := database.C.Delete(&subscription).Error
	return err
}

func UnsubscribeFromCategory(user authm.Account, target models.Category) error {
	var subscription models.Subscription
	if err := database.C.Where("follower_id = ? AND category_id = ?", user.ID, target.ID).First(&subscription).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("subscription does not exist")
		}
		return fmt.Errorf("unable to check subscription is exists or not: %v", err)
	}

	err := database.C.Delete(&subscription).Error
	return err
}

func NotifyUserSubscription(poster models.Publisher, item models.Post, content string, title *string) error {
	if item.Visibility == models.PostVisibilityNone {
		return nil
	}

	var subscriptions []models.Subscription
	if err := database.C.Where("account_id = ?", poster.ID).Preload("Follower").Find(&subscriptions).Error; err != nil {
		return fmt.Errorf("unable to get subscriptions: %v", err)
	}

	nTitle := fmt.Sprintf("New post from %s (%s)", poster.Nick, poster.Name)
	nSubtitle := "From your subscription"

	body := TruncatePostContentShort(content)
	if title != nil {
		body = fmt.Sprintf("%s\n%s", *title, body)
	}

	userIDs := make([]uint64, 0, len(subscriptions))
	for _, subscription := range subscriptions {
		userIDs = append(userIDs, uint64(subscription.Follower.ID))
	}

	if item.Visibility == models.PostVisibilitySelected {
		userIDs = lo.Filter(userIDs, func(entry uint64, index int) bool {
			return lo.Contains(item.VisibleUsers, uint(entry))
		})
	} else if item.Visibility == models.PostVisibilityFiltered {
		userIDs = lo.Filter(userIDs, func(entry uint64, index int) bool {
			return !lo.Contains(item.InvisibleUsers, uint(entry))
		})
	} else if invisibleList := ListPostInvisibleUser(poster, item.Visibility); len(invisibleList) > 0 {
		userIDs = lo.Filter(userIDs, func(entry uint64, index int) bool {
			return !lo.Contains(invisibleList, uint(entry))
		})
	}

	err := authkit.NotifyUserBatch(gap.Nx, userIDs, pushkit.Notification{
		Topic:    "interactive.subscription",
		Title:    nTitle,
		Subtitle: nSubtitle,
		Body:     body,
		Priority: 3,
	})

	return err
}

func NotifyTagSubscription(poster models.Tag, og models.Publisher, item models.Post, content string, title *string) error {
	if item.Visibility == models.PostVisibilityNone {
		return nil
	}

	var subscriptions []models.Subscription
	if err := database.C.Where("tag_id = ?", poster.ID).Preload("Follower").Find(&subscriptions).Error; err != nil {
		return fmt.Errorf("unable to get subscriptions: %v", err)
	}

	nTitle := fmt.Sprintf("New post in %s by %s (%s)", poster.Name, og.Nick, og.Name)
	nSubtitle := "From your subscription"

	body := TruncatePostContentShort(content)
	if title != nil {
		body = fmt.Sprintf("%s\n%s", *title, body)
	}

	userIDs := make([]uint64, 0, len(subscriptions))
	for _, subscription := range subscriptions {
		userIDs = append(userIDs, uint64(subscription.Follower.ID))
	}

	if item.Visibility == models.PostVisibilitySelected {
		userIDs = lo.Filter(userIDs, func(entry uint64, index int) bool {
			return lo.Contains(item.VisibleUsers, uint(entry))
		})
	} else if item.Visibility == models.PostVisibilityFiltered {
		userIDs = lo.Filter(userIDs, func(entry uint64, index int) bool {
			return !lo.Contains(item.InvisibleUsers, uint(entry))
		})
	} else if invisibleList := ListPostInvisibleUser(item.Publisher, item.Visibility); len(invisibleList) > 0 {
		userIDs = lo.Filter(userIDs, func(entry uint64, index int) bool {
			return !lo.Contains(invisibleList, uint(entry))
		})
	}

	err := authkit.NotifyUserBatch(gap.Nx, userIDs, pushkit.Notification{
		Topic:    "interactive.subscription",
		Title:    nTitle,
		Subtitle: nSubtitle,
		Body:     body,
		Priority: 3,
	})

	return err
}

func NotifyCategorySubscription(poster models.Category, og models.Publisher, item models.Post, content string, title *string) error {
	if item.Visibility == models.PostVisibilityNone {
		return nil
	}

	var subscriptions []models.Subscription
	if err := database.C.Where("category_id = ?", poster.ID).Preload("Follower").Find(&subscriptions).Error; err != nil {
		return fmt.Errorf("unable to get subscriptions: %v", err)
	}

	nTitle := fmt.Sprintf("New post in %s by %s (%s)", poster.Name, og.Nick, og.Name)
	nSubtitle := "From your subscription"

	body := TruncatePostContentShort(content)
	if title != nil {
		body = fmt.Sprintf("%s\n%s", *title, body)
	}

	userIDs := make([]uint64, 0, len(subscriptions))
	for _, subscription := range subscriptions {
		userIDs = append(userIDs, uint64(subscription.Follower.ID))
	}

	if item.Visibility == models.PostVisibilitySelected {
		userIDs = lo.Filter(userIDs, func(entry uint64, index int) bool {
			return lo.Contains(item.VisibleUsers, uint(entry))
		})
	} else if item.Visibility == models.PostVisibilityFiltered {
		userIDs = lo.Filter(userIDs, func(entry uint64, index int) bool {
			return !lo.Contains(item.InvisibleUsers, uint(entry))
		})
	} else if invisibleList := ListPostInvisibleUser(item.Publisher, item.Visibility); len(invisibleList) > 0 {
		userIDs = lo.Filter(userIDs, func(entry uint64, index int) bool {
			return !lo.Contains(invisibleList, uint(entry))
		})
	}

	err := authkit.NotifyUserBatch(gap.Nx, userIDs, pushkit.Notification{
		Topic:    "interactive.subscription",
		Title:    nTitle,
		Subtitle: nSubtitle,
		Body:     body,
		Priority: 3,
	})

	return err
}

// ListPostInvisibleUser will return a list of users which should not be notified the post.
// NOTICE If the visibility is PostVisibilitySelected, PostVisibilityFiltered or PostVisibilityNone, you need do extra steps to filter users
// WARNING This function won't use cache, be careful of the queries
func ListPostInvisibleUser(og models.Publisher, visibility models.PostVisibilityLevel) []uint {
	switch visibility {
	case models.PostVisibilityAll:
		return []uint{}
	case models.PostVisibilityFriends:
		if og.AccountID == nil {
			return []uint{}
		}
		userFriends, _ := authkit.ListRelative(gap.Nx, *og.AccountID, int32(authm.RelationshipFriend), true)
		return lo.Map(userFriends, func(item *proto.UserInfo, index int) uint {
			return uint(item.GetId())
		})
	default:
		return nil
	}
}
