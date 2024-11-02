package services

import (
	"fmt"

	"git.solsynth.dev/hypernet/interactive/pkg/internal/models"
	"github.com/samber/lo"
	"gorm.io/gorm"
)

func ListPostReactions(tx *gorm.DB) (map[string]int64, error) {
	var reactions []struct {
		Symbol string
		Count  int64
	}

	if err := tx.Model(&models.Reaction{}).
		Select("symbol, COUNT(id) as count").
		Group("symbol").
		Scan(&reactions).Error; err != nil {
		return map[string]int64{}, err
	}

	return lo.SliceToMap(reactions, func(item struct {
		Symbol string
		Count  int64
	},
	) (string, int64) {
		return item.Symbol, item.Count
	}), nil
}

func BatchListPostReactions(tx *gorm.DB, indexField string) (map[uint]map[string]int64, error) {
	var reactions []struct {
		ID     uint
		Symbol string
		Count  int64
	}

	reactInfo := map[uint]map[string]int64{}
	if err := tx.Model(&models.Reaction{}).
		Select(fmt.Sprintf("%s as id, symbol, COUNT(*) as count", indexField)).
		Group("id, symbol").
		Scan(&reactions).Error; err != nil {
		return reactInfo, err
	}

	for _, info := range reactions {
		if _, ok := reactInfo[info.ID]; !ok {
			reactInfo[info.ID] = make(map[string]int64)
		}
		if _, exists := reactInfo[info.ID][info.Symbol]; exists {
			reactInfo[info.ID][info.Symbol] += info.Count
		} else {
			reactInfo[info.ID][info.Symbol] = info.Count
		}
	}

	return reactInfo, nil
}
