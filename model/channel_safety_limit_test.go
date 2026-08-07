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

func TestChannelIsExplicitlySafe(t *testing.T) {
	tests := []struct {
		name string
		tag  *string
		safe bool
	}{
		{name: "exact safe tag", tag: common.GetPointer(ChannelTagSafe), safe: true},
		{name: "trimmed safe tag", tag: common.GetPointer("  安全 \t"), safe: true},
		{name: "unverified tag", tag: common.GetPointer("无法验证安全性"), safe: false},
		{name: "empty tag", tag: common.GetPointer(""), safe: false},
		{name: "other tag", tag: common.GetPointer("内部"), safe: false},
		{name: "nil tag", tag: nil, safe: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			channel := &Channel{Tag: test.tag}
			assert.Equal(t, test.safe, channel.IsExplicitlySafe())
		})
	}
}

func TestChannelSafetyPolicyFiltersBeforePriorityAndPreservesErrorPrecedence(t *testing.T) {
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
			unsafeTag := "无法验证安全性"
			safeTag := " 安全 "
			unsafeRatio := 1.0
			safeRatio := 2.0
			weight := uint(100)
			channels := []Channel{
				{Id: 301, Name: "unsafe-high-priority", Status: common.ChannelStatusEnabled, Models: "safety-model", Group: "default", Priority: &highPriority, Weight: &weight, Ratio: &unsafeRatio, Tag: &unsafeTag},
				{Id: 302, Name: "safe-low-priority", Status: common.ChannelStatusEnabled, Models: "safety-model", Group: "default", Priority: &lowPriority, Weight: &weight, Ratio: &safeRatio, Tag: &safeTag},
			}
			require.NoError(t, db.Create(&channels).Error)
			require.NoError(t, db.Create(&[]Ability{
				{Group: "default", Model: "safety-model", ChannelId: 301, Enabled: true, Priority: &highPriority, Weight: weight},
				{Group: "default", Model: "safety-model", ChannelId: 302, Enabled: true, Priority: &lowPriority, Weight: weight},
			}).Error)
			if memoryCacheEnabled {
				InitChannelCache()
			}

			safeOnlyPolicy := ChannelSelectionPolicy{AllowUnsafeChannels: false}
			channel, err := GetRandomSatisfiedChannelWithSelectionPolicy("default", "safety-model", 0, "", nil, safeOnlyPolicy)
			require.NoError(t, err)
			require.NotNil(t, channel)
			assert.Equal(t, 302, channel.Id)

			channel, err = GetRandomSatisfiedChannelWithSelectionPolicy("default", "safety-model", 0, "", nil, ChannelSelectionPolicy{AllowUnsafeChannels: true})
			require.NoError(t, err)
			require.NotNil(t, channel)
			assert.Equal(t, 301, channel.Id)

			pricingPolicy := ChannelSelectionPolicy{
				AllowUnsafeChannels: false,
				PricingLimit: ChannelPricingLimit{
					MaxChannelRatio: 1.5,
					ratioOnly:       true,
				},
			}
			channel, err = GetRandomSatisfiedChannelWithSelectionPolicy("default", "safety-model", 0, "", nil, pricingPolicy)
			assert.Nil(t, channel)
			var pricingLimitErr *ChannelPricingLimitError
			require.True(t, errors.As(err, &pricingLimitErr))

			channel, err = GetRandomSatisfiedChannelWithSelectionPolicy("default", "safety-model", 0, "", map[int]struct{}{302: {}}, safeOnlyPolicy)
			assert.Nil(t, channel)
			var safetyLimitErr *ChannelSafetyLimitError
			require.True(t, errors.As(err, &safetyLimitErr))
		})
	}
}
