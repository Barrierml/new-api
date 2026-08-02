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

func TestChannelRatioLimitFiltersBeforePrioritySelection(t *testing.T) {
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

			channel, err := GetRandomSatisfiedChannelWithRatioLimit("default", "ratio-model", 0, "", nil, 1.0)
			require.NoError(t, err)
			require.NotNil(t, channel)
			assert.Equal(t, 102, channel.Id)

			channel, err = GetRandomSatisfiedChannelWithRatioLimit("default", "ratio-model", 0, "", map[int]struct{}{102: {}}, 1.5)
			assert.Nil(t, channel)
			var ratioLimitErr *ChannelRatioLimitError
			require.True(t, errors.As(err, &ratioLimitErr))
			assert.Equal(t, 1.5, ratioLimitErr.MaxChannelRatio)
			assert.Equal(t, 2.0, ratioLimitErr.MinAvailableRatio)
		})
	}
}
