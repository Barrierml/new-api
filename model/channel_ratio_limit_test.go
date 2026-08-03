package model

import (
	"errors"
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestChannelPricingLimitTruthTable(t *testing.T) {
	baseLimit := ChannelPricingLimit{
		MaxChannelRatio:      10,
		MaxInputPrice:        20,
		BaseInputPrice:       2,
		GroupRatio:           1,
		InputPriceComparable: true,
	}
	tests := []struct {
		name         string
		basePrice    float64
		channelRatio float64
		blocked      bool
	}{
		{name: "neither limit exceeded", basePrice: 1, channelRatio: 5, blocked: false},
		{name: "only channel ratio exceeded", basePrice: 1, channelRatio: 11, blocked: false},
		{name: "only input price exceeded", basePrice: 30, channelRatio: 1, blocked: false},
		{name: "both limits exceeded", basePrice: 2, channelRatio: 11, blocked: true},
		{name: "equal limits are allowed", basePrice: 2, channelRatio: 10, blocked: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			limit := baseLimit
			limit.BaseInputPrice = test.basePrice
			assert.Equal(t, test.blocked, limit.Evaluate(test.channelRatio).Blocked)
		})
	}

	noInputPriceLimit := baseLimit
	noInputPriceLimit.MaxInputPrice = 0
	assert.False(t, noInputPriceLimit.Evaluate(1000).Blocked)

	unresolvedPrice := baseLimit
	unresolvedPrice.InputPriceComparable = false
	assert.True(t, unresolvedPrice.Evaluate(20).Blocked)
	unresolvedPrice.MaxInputPrice = 0
	assert.False(t, unresolvedPrice.Evaluate(20).Blocked)

	ratioOnly := ChannelPricingLimit{MaxChannelRatio: 10, ratioOnly: true}
	assert.True(t, ratioOnly.Evaluate(20).Blocked)
}

func TestChannelPricingLimitFiltersBeforePrioritySelection(t *testing.T) {
	for _, memoryCacheEnabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("memory_cache_%t", memoryCacheEnabled), func(t *testing.T) {
			previousDB := DB
			previousMemoryCacheEnabled := common.MemoryCacheEnabled
			previousMainDatabaseType := common.MainDatabaseType()

			db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
			require.NoError(t, err)
			require.NoError(t, db.AutoMigrate(&Channel{}, &Ability{}))
			DB = db
			common.MemoryCacheEnabled = memoryCacheEnabled
			common.SetMainDatabaseType(common.DatabaseTypeSQLite)
			t.Cleanup(func() {
				DB = previousDB
				common.MemoryCacheEnabled = previousMemoryCacheEnabled
				common.SetMainDatabaseType(previousMainDatabaseType)
			})

			highPriority := int64(100)
			lowPriority := int64(50)
			expensiveRatio := 2.0
			cheapRatio := 1.0
			weight := uint(100)
			channels := []Channel{
				{Id: 101, Name: "expensive", Status: common.ChannelStatusEnabled, Models: "ratio-model", Group: "default", Priority: &highPriority, Weight: &weight, Ratio: &expensiveRatio},
				{Id: 102, Name: "cheap", Status: common.ChannelStatusEnabled, Models: "ratio-model", Group: "default", Priority: &lowPriority, Weight: &weight, Ratio: &cheapRatio},
			}
			require.NoError(t, db.Create(&channels).Error)
			require.NoError(t, db.Create(&[]Ability{
				{Group: "default", Model: "ratio-model", ChannelId: 101, Enabled: true, Priority: &highPriority, Weight: weight},
				{Group: "default", Model: "ratio-model", ChannelId: 102, Enabled: true, Priority: &lowPriority, Weight: weight},
			}).Error)
			if memoryCacheEnabled {
				InitChannelCache()
			}

			pricingLimit := ChannelPricingLimit{
				MaxChannelRatio:      1.5,
				MaxInputPrice:        1,
				BaseInputPrice:       0.1,
				GroupRatio:           1,
				InputPriceComparable: true,
			}
			channel, err := GetRandomSatisfiedChannelWithPricingLimit("default", "ratio-model", 0, "", nil, pricingLimit)
			require.NoError(t, err)
			require.NotNil(t, channel)
			assert.Equal(t, 101, channel.Id)

			pricingLimit.MaxInputPrice = 0.15
			channel, err = GetRandomSatisfiedChannelWithPricingLimit("default", "ratio-model", 0, "", nil, pricingLimit)
			require.NoError(t, err)
			require.NotNil(t, channel)
			assert.Equal(t, 102, channel.Id)

			channel, err = GetRandomSatisfiedChannelWithPricingLimit("default", "ratio-model", 0, "", map[int]struct{}{102: {}}, pricingLimit)
			assert.Nil(t, channel)
			var pricingLimitErr *ChannelPricingLimitError
			require.True(t, errors.As(err, &pricingLimitErr))
			assert.Equal(t, 1.5, pricingLimitErr.MaxChannelRatio)
			assert.Equal(t, 0.15, pricingLimitErr.MaxInputPrice)
			assert.Equal(t, 2.0, pricingLimitErr.MinAvailableRatio)
			assert.Equal(t, 0.2, pricingLimitErr.MinActualInputPrice)
		})
	}
}
