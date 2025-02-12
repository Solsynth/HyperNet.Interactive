package services

import (
	"fmt"

	"git.solsynth.dev/hypernet/interactive/pkg/internal/database"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/models"
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

	var count int64
	if err := database.C.Model(&models.PollAnswer{}).Where("poll_id = ? AND account_id = ?", poll.ID, answer.AccountID).Count(&count).Error; err != nil {
		return answer, fmt.Errorf("you already answered the poll")
	}
	if err := database.C.Create(&answer).Error; err != nil {
		return answer, err
	}

	return answer, nil
}

func GetPollMetric(poll models.Poll) models.PollMetric {
	var answers []models.PollAnswer
	if err := database.C.Where("poll_id = ?", poll.ID).Find(&answers); err != nil {
		return models.PollMetric{}
	}

	byOptions := make(map[string]int64)
	for _, answer := range answers {
		if _, ok := byOptions[answer.Answer]; !ok {
			byOptions[answer.Answer] = 0
		}
		byOptions[answer.Answer]++
	}

	return models.PollMetric{
		TotalAnswer: int64(len(answers)),
		ByOptions:   byOptions,
	}
}
