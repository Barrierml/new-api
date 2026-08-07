package model

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"

	"github.com/samber/lo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Ability struct {
	Group     string  `json:"group" gorm:"type:varchar(64);primaryKey;autoIncrement:false"`
	Model     string  `json:"model" gorm:"type:varchar(255);primaryKey;autoIncrement:false"`
	ChannelId int     `json:"channel_id" gorm:"primaryKey;autoIncrement:false;index"`
	Enabled   bool    `json:"enabled"`
	Priority  *int64  `json:"priority" gorm:"bigint;default:0;index"`
	Weight    uint    `json:"weight" gorm:"default:0;index"`
	Tag       *string `json:"tag" gorm:"index"`
}

type AbilityWithChannel struct {
	Ability
	ChannelType   int     `json:"channel_type"`
	ChannelName   string  `json:"channel_name"`
	ChannelRatio  float64 `json:"channel_ratio"`
	ChannelWeight uint    `json:"channel_weight"`
}

func GetAllEnableAbilityWithChannels() ([]AbilityWithChannel, error) {
	var abilities []AbilityWithChannel
	err := DB.Table("abilities").
		Select("abilities.*, channels.type as channel_type, channels.name as channel_name, COALESCE(channels.ratio, 1) as channel_ratio, COALESCE(channels.weight, 0) as channel_weight").
		Joins("left join channels on abilities.channel_id = channels.id").
		Where("abilities.enabled = ? AND channels.status = ?", true, common.ChannelStatusEnabled).
		Scan(&abilities).Error
	return abilities, err
}

func GetGroupEnabledModels(group string) []string {
	var models []string
	// Find distinct models
	DB.Table("abilities").Where(commonGroupCol+" = ? and enabled = ?", group, true).Distinct("model").Pluck("model", &models)
	return models
}

func GetEnabledModels() []string {
	var models []string
	// Find distinct models
	DB.Table("abilities").Where("enabled = ?", true).Distinct("model").Pluck("model", &models)
	return models
}

func GetAllEnableAbilities() []Ability {
	var abilities []Ability
	DB.Find(&abilities, "enabled = ?", true)
	return abilities
}

func channelIDsFromSet(excluded map[int]struct{}) []int {
	if len(excluded) == 0 {
		return nil
	}
	ids := make([]int, 0, len(excluded))
	for channelID := range excluded {
		ids = append(ids, channelID)
	}
	return ids
}

func GetChannel(group string, model string, retry int, requestPath string) (*Channel, error) {
	return GetChannelWithExclusions(group, model, retry, requestPath, nil)
}

func GetChannelWithExclusions(group string, model string, retry int, requestPath string, excluded map[int]struct{}) (*Channel, error) {
	return GetChannelWithSelectionPolicy(group, model, retry, requestPath, excluded, ChannelSelectionPolicy{AllowUnsafeChannels: true})
}

func GetChannelWithRatioLimit(group string, model string, retry int, requestPath string, excluded map[int]struct{}, maxChannelRatio float64) (*Channel, error) {
	return GetChannelWithSelectionPolicy(group, model, retry, requestPath, excluded, ChannelSelectionPolicy{
		AllowUnsafeChannels: true,
		PricingLimit: ChannelPricingLimit{
			MaxChannelRatio: maxChannelRatio,
			ratioOnly:       true,
		},
	})
}

func GetChannelWithPricingLimit(group string, model string, retry int, requestPath string, excluded map[int]struct{}, pricingLimit ChannelPricingLimit) (*Channel, error) {
	return GetChannelWithSelectionPolicy(group, model, retry, requestPath, excluded, ChannelSelectionPolicy{
		PricingLimit:        pricingLimit,
		AllowUnsafeChannels: true,
	})
}

func GetChannelWithSelectionPolicy(group string, model string, retry int, requestPath string, excluded map[int]struct{}, policy ChannelSelectionPolicy) (*Channel, error) {
	var abilities []Ability
	query := DB.Where(commonGroupCol+" = ? and model = ? and enabled = ?", group, model, true)
	if excludedIDs := channelIDsFromSet(excluded); len(excludedIDs) > 0 {
		query = query.Where("channel_id NOT IN ?", excludedIDs)
	}
	if err := query.Find(&abilities).Error; err != nil {
		return nil, err
	}
	if len(abilities) == 0 {
		return nil, nil
	}

	channelIDs := make([]int, 0, len(abilities))
	for _, ability := range abilities {
		channelIDs = append(channelIDs, ability.ChannelId)
	}
	var channels []*Channel
	if err := DB.Where("id IN ?", channelIDs).Find(&channels).Error; err != nil {
		return nil, err
	}
	channelsByID := make(map[int]*Channel, len(channels))
	for _, channel := range channels {
		channelsByID[channel.Id] = channel
	}

	eligible := make([]Ability, 0, len(abilities))
	var pricingLimitErr *ChannelPricingLimitError
	safetyBlocked := false
	for _, ability := range abilities {
		channel, ok := channelsByID[ability.ChannelId]
		if !ok {
			return nil, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", ability.ChannelId)
		}
		if requestPath != "" && channel.Type == constant.ChannelTypeAdvancedCustom {
			config := channel.GetOtherSettings().AdvancedCustom
			if config == nil || !config.SupportsPathForModel(requestPath, model) {
				continue
			}
		}
		policyResult := policy.Evaluate(channel)
		if policyResult.SafetyBlocked {
			safetyBlocked = true
			continue
		}
		if policyResult.Blocked {
			if pricingLimitErr == nil {
				pricingLimitErr = NewChannelPricingLimitError(policy.PricingLimit, policyResult.PricingResult)
			} else {
				pricingLimitErr.Consider(policyResult.PricingResult)
			}
			continue
		}
		eligible = append(eligible, ability)
	}
	if len(eligible) == 0 {
		if pricingLimitErr != nil {
			return nil, pricingLimitErr
		}
		if safetyBlocked {
			return nil, &ChannelSafetyLimitError{}
		}
		return nil, nil
	}

	priorities := make(map[int64]struct{})
	for _, ability := range eligible {
		priorities[channelsByID[ability.ChannelId].GetPriority()] = struct{}{}
	}
	sortedPriorities := make([]int64, 0, len(priorities))
	for priority := range priorities {
		sortedPriorities = append(sortedPriorities, priority)
	}
	sort.Slice(sortedPriorities, func(i, j int) bool {
		return sortedPriorities[i] > sortedPriorities[j]
	})
	if retry >= len(sortedPriorities) {
		retry = len(sortedPriorities) - 1
	}
	targetPriority := sortedPriorities[retry]

	targetAbilities := make([]Ability, 0, len(eligible))
	weightSum := uint(0)
	for _, ability := range eligible {
		if channelsByID[ability.ChannelId].GetPriority() == targetPriority {
			targetAbilities = append(targetAbilities, ability)
			weightSum += ability.Weight + 10
		}
	}
	weight := common.GetRandomInt(int(weightSum))
	for _, ability := range targetAbilities {
		weight -= int(ability.Weight) + 10
		if weight <= 0 {
			return channelsByID[ability.ChannelId], nil
		}
	}
	return nil, errors.New("channel not found")
}

func (channel *Channel) AddAbilities(tx *gorm.DB) error {
	models_ := strings.Split(channel.Models, ",")
	groups_ := strings.Split(channel.Group, ",")
	abilitySet := make(map[string]struct{})
	abilities := make([]Ability, 0, len(models_))
	for _, model := range models_ {
		for _, group := range groups_ {
			key := group + "|" + model
			if _, exists := abilitySet[key]; exists {
				continue
			}
			abilitySet[key] = struct{}{}
			ability := Ability{
				Group:     group,
				Model:     model,
				ChannelId: channel.Id,
				Enabled:   channel.Status == common.ChannelStatusEnabled,
				Priority:  channel.Priority,
				Weight:    uint(channel.GetWeight()),
				Tag:       channel.Tag,
			}
			abilities = append(abilities, ability)
		}
	}
	if len(abilities) == 0 {
		return nil
	}
	// choose DB or provided tx
	useDB := DB
	if tx != nil {
		useDB = tx
	}
	for _, chunk := range lo.Chunk(abilities, 50) {
		err := useDB.Clauses(clause.OnConflict{DoNothing: true}).Create(&chunk).Error
		if err != nil {
			return err
		}
	}
	return nil
}

func (channel *Channel) DeleteAbilities() error {
	return DB.Where("channel_id = ?", channel.Id).Delete(&Ability{}).Error
}

// UpdateAbilities updates abilities of this channel.
// Make sure the channel is completed before calling this function.
func (channel *Channel) UpdateAbilities(tx *gorm.DB) error {
	isNewTx := false
	// 如果没有传入事务，创建新的事务
	if tx == nil {
		tx = DB.Begin()
		if tx.Error != nil {
			return tx.Error
		}
		isNewTx = true
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
			}
		}()
	}

	// First delete all abilities of this channel
	err := tx.Where("channel_id = ?", channel.Id).Delete(&Ability{}).Error
	if err != nil {
		if isNewTx {
			tx.Rollback()
		}
		return err
	}

	// Then add new abilities
	models_ := strings.Split(channel.Models, ",")
	groups_ := strings.Split(channel.Group, ",")
	abilitySet := make(map[string]struct{})
	abilities := make([]Ability, 0, len(models_))
	for _, model := range models_ {
		for _, group := range groups_ {
			key := group + "|" + model
			if _, exists := abilitySet[key]; exists {
				continue
			}
			abilitySet[key] = struct{}{}
			ability := Ability{
				Group:     group,
				Model:     model,
				ChannelId: channel.Id,
				Enabled:   channel.Status == common.ChannelStatusEnabled,
				Priority:  channel.Priority,
				Weight:    uint(channel.GetWeight()),
				Tag:       channel.Tag,
			}
			abilities = append(abilities, ability)
		}
	}

	if len(abilities) > 0 {
		for _, chunk := range lo.Chunk(abilities, 50) {
			err = tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&chunk).Error
			if err != nil {
				if isNewTx {
					tx.Rollback()
				}
				return err
			}
		}
	}

	// 如果是新创建的事务，需要提交
	if isNewTx {
		return tx.Commit().Error
	}

	return nil
}

func UpdateAbilityStatus(channelId int, status bool) error {
	return DB.Model(&Ability{}).Where("channel_id = ?", channelId).Select("enabled").Update("enabled", status).Error
}

func UpdateAbilityStatusByTag(tag string, status bool) error {
	return DB.Model(&Ability{}).Where("tag = ?", tag).Select("enabled").Update("enabled", status).Error
}

func UpdateAbilityByTag(tag string, newTag *string, priority *int64, weight *uint) error {
	ability := Ability{}
	if newTag != nil {
		ability.Tag = newTag
	}
	if priority != nil {
		ability.Priority = priority
	}
	if weight != nil {
		ability.Weight = *weight
	}
	return DB.Model(&Ability{}).Where("tag = ?", tag).Updates(ability).Error
}

var fixLock = sync.Mutex{}

func FixAbility() (int, int, error) {
	lock := fixLock.TryLock()
	if !lock {
		return 0, 0, errors.New("已经有一个修复任务在运行中，请稍后再试")
	}
	defer fixLock.Unlock()

	// truncate abilities table
	if common.UsingMainDatabase(common.DatabaseTypeSQLite) {
		err := DB.Exec("DELETE FROM abilities").Error
		if err != nil {
			common.SysLog(fmt.Sprintf("Delete abilities failed: %s", err.Error()))
			return 0, 0, err
		}
	} else {
		err := DB.Exec("TRUNCATE TABLE abilities").Error
		if err != nil {
			common.SysLog(fmt.Sprintf("Truncate abilities failed: %s", err.Error()))
			return 0, 0, err
		}
	}
	var channels []*Channel
	// Find all channels
	err := DB.Model(&Channel{}).Find(&channels).Error
	if err != nil {
		return 0, 0, err
	}
	if len(channels) == 0 {
		return 0, 0, nil
	}
	successCount := 0
	failCount := 0
	for _, chunk := range lo.Chunk(channels, 50) {
		ids := lo.Map(chunk, func(c *Channel, _ int) int { return c.Id })
		// Delete all abilities of this channel
		err = DB.Where("channel_id IN ?", ids).Delete(&Ability{}).Error
		if err != nil {
			common.SysLog(fmt.Sprintf("Delete abilities failed: %s", err.Error()))
			failCount += len(chunk)
			continue
		}
		// Then add new abilities
		for _, channel := range chunk {
			err = channel.AddAbilities(nil)
			if err != nil {
				common.SysLog(fmt.Sprintf("Add abilities for channel %d failed: %s", channel.Id, err.Error()))
				failCount++
			} else {
				successCount++
			}
		}
	}
	InitChannelCache()
	return successCount, failCount, nil
}
