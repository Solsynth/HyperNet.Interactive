package services

import (
	"errors"
	"fmt"

	"git.solsynth.dev/hypernet/interactive/pkg/internal/database"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/models"
	"gorm.io/gorm"
)

func NewPoll(poll models.Poll) (models.Poll, error) {
	if err := database.C.Create(&poll).Error; err != nil {
		return poll, err
	}
	return poll, nil
}

func UpdatePoll(poll models.Poll) (models.Poll, error) {
	if err := database.C.Save(&poll).Error; err != nil {
		return poll, err
	}
	return poll, nil
}

func AddPollAnswer(poll models.Poll, answer models.PollAnswer) (models.PollAnswer, error) {
	answer.PollID = poll.ID

	var currentAnswer models.PollAnswer
	if err := database.C.Model(&models.PollAnswer{}).
		Where("poll_id = ? AND account_id = ?", poll.ID, answer.AccountID).
		First(&currentAnswer).Error; err == nil {
		if err := database.C.Model(&currentAnswer).
			Where("id = ?", currentAnswer.ID).
			Updates(&models.PollAnswer{Answer: answer.Answer}).Error; err != nil {
			return answer, fmt.Errorf("failed to update your answer")
		}

		return answer, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return answer, err
	}

	if err := database.C.Create(&answer).Error; err != nil {
		return answer, err
	}

	return answer, nil
}

func GetPollMetric(poll models.Poll) models.PollMetric {
	var answers []models.PollAnswer
	if err := database.C.Where("poll_id = ?", poll.ID).Find(&answers).Error; err != nil {
		return models.PollMetric{}
	}

	byOptions := make(map[string]int64)
	for _, answer := range answers {
		if _, ok := byOptions[answer.Answer]; !ok {
			byOptions[answer.Answer] = 0
		}
		byOptions[answer.Answer]++
	}

	byOptionsPercentage := make(map[string]float64)
	for _, option := range poll.Options {
		if val, ok := byOptions[option.ID]; ok {
			byOptionsPercentage[option.ID] = float64(val) / float64(len(answers))
		}
	}

	return models.PollMetric{
		TotalAnswer:         int64(len(answers)),
		ByOptions:           byOptions,
		ByOptionsPercentage: byOptionsPercentage,
	}
}
