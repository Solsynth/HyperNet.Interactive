package services

import (
	"fmt"

	"git.solsynth.dev/hypernet/interactive/pkg/internal/database"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/gap"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/models"
	"git.solsynth.dev/hypernet/paperclip/pkg/filekit"
	"git.solsynth.dev/hypernet/paperclip/pkg/proto"
	authm "git.solsynth.dev/hypernet/passport/pkg/authkit/models"
)

func GetPublisher(id uint, userID uint) (models.Publisher, error) {
	var publisher models.Publisher
	if err := database.C.Where("id = ? AND account_id = ?", id, userID).First(&publisher).Error; err != nil {
		return publisher, fmt.Errorf("unable to get publisher: %v", err)
	}
	return publisher, nil
}

func GetPublisherByName(name string, userID uint) (models.Publisher, error) {
	var publisher models.Publisher
	if err := database.C.Where("name = ? AND account_id = ?", name, userID).First(&publisher).Error; err != nil {
		return publisher, fmt.Errorf("unable to get publisher: %v", err)
	}
	return publisher, nil
}

func CreatePersonalPublisher(user authm.Account, name, nick, desc, avatar, banner string) (models.Publisher, error) {
	publisher := models.Publisher{
		Type:        models.PublisherTypePersonal,
		Name:        name,
		Nick:        nick,
		Description: desc,
		Avatar:      avatar,
		Banner:      banner,
		AccountID:   &user.ID,
	}
	var attachments []string
	if user.Avatar != nil && len(publisher.Avatar) == 0 {
		attachments = append(attachments, *user.Avatar)
		publisher.Avatar = *user.Avatar
	}
	if user.Banner != nil && len(publisher.Banner) == 0 {
		attachments = append(attachments, *user.Banner)
		publisher.Banner = *user.Banner
	}

	if len(attachments) > 0 {
		filekit.CountAttachmentUsage(gap.Nx, &proto.UpdateUsageRequest{
			Rid:   attachments,
			Delta: 1,
		})
	}

	if err := database.C.Create(&publisher).Error; err != nil {
		return publisher, err
	}
	return publisher, nil
}

func CreateOrganizationPublisher(user authm.Account, realm authm.Realm, name, nick, desc, avatar, banner string) (models.Publisher, error) {
	publisher := models.Publisher{
		Type:        models.PublisherTypeOrganization,
		Name:        name,
		Nick:        nick,
		Description: desc,
		Avatar:      avatar,
		Banner:      banner,
		RealmID:     &realm.ID,
		AccountID:   &user.ID,
	}
	var attachments []string
	if realm.Avatar != nil && len(publisher.Avatar) == 0 {
		attachments = append(attachments, *realm.Avatar)
		publisher.Avatar = *realm.Avatar
	}
	if realm.Banner != nil && len(publisher.Banner) == 0 {
		attachments = append(attachments, *realm.Banner)
		publisher.Banner = *realm.Banner
	}

	if len(attachments) > 0 {
		filekit.CountAttachmentUsage(gap.Nx, &proto.UpdateUsageRequest{
			Rid:   attachments,
			Delta: 1,
		})
	}

	if err := database.C.Create(&publisher).Error; err != nil {
		return publisher, err
	}
	return publisher, nil
}

func EditPublisher(user authm.Account, publisher, og models.Publisher) (models.Publisher, error) {
	if publisher.Type == models.PublisherTypePersonal {
		if *publisher.AccountID != user.ID {
			return publisher, fmt.Errorf("you cannot transfer personal publisher")
		}
	}

	var minusAttachments, plusAttachments []string
	if publisher.Avatar != og.Avatar {
		minusAttachments = append(minusAttachments, og.Avatar)
		plusAttachments = append(plusAttachments, publisher.Avatar)
	}
	if publisher.Banner != og.Banner {
		minusAttachments = append(minusAttachments, og.Banner)
		plusAttachments = append(plusAttachments, publisher.Banner)
	}
	if len(minusAttachments) > 0 {
		filekit.CountAttachmentUsage(gap.Nx, &proto.UpdateUsageRequest{
			Rid:   minusAttachments,
			Delta: -1,
		})
	}
	if len(plusAttachments) > 0 {
		filekit.CountAttachmentUsage(gap.Nx, &proto.UpdateUsageRequest{
			Rid:   plusAttachments,
			Delta: 1,
		})
	}

	err := database.C.Save(&publisher).Error
	return publisher, err
}

func DeletePublisher(publisher models.Publisher) error {
	tx := database.C.Begin()

	if err := tx.Where("publisher_id = ?", publisher.ID).Delete(&models.Post{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Delete(&publisher).Error; err != nil {
		tx.Rollback()
		return err
	}

	err := tx.Commit().Error
	if err == nil {
		var attachments []string
		if len(publisher.Avatar) > 0 {
			attachments = append(attachments, publisher.Avatar)
		}
		if len(publisher.Banner) > 0 {
			attachments = append(attachments, publisher.Banner)
		}
		if len(attachments) > 0 {
			filekit.CountAttachmentUsage(gap.Nx, &proto.UpdateUsageRequest{
				Rid:   attachments,
				Delta: -1,
			})
		}
	}

	return err
}
