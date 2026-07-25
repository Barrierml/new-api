package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
)

func TestPickOperationsWindows(t *testing.T) {
	hourUsage := model.SubscriptionSubQuotaUsage{
		PeriodUnit: "hour", LimitUSD: 90, UsedUSD: 30, Percent: 33.3, WindowEnd: 1000,
	}
	dayUsage := model.SubscriptionSubQuotaUsage{
		PeriodUnit: "day", LimitUSD: 24, UsedUSD: 5, Percent: 20.8, WindowEnd: 2000,
	}
	weekUsage := model.SubscriptionSubQuotaUsage{
		PeriodUnit: "week", LimitUSD: 240, UsedUSD: 100, Percent: 41.7, WindowEnd: 3000,
	}
	mainUsage := &model.MainQuotaUsage{LimitUSD: 500, UsedUSD: 200, Percent: 40, ResetTime: 4000}

	tests := []struct {
		name       string
		main       *model.MainQuotaUsage
		subs       []model.SubscriptionSubQuotaUsage
		want5h     *operationsWindow
		wantWeekly *operationsWindow
	}{
		{
			name: "hour+week 子限制各就各位",
			main: mainUsage,
			subs: []model.SubscriptionSubQuotaUsage{hourUsage, dayUsage, weekUsage},
			want5h: &operationsWindow{LimitUSD: 90, UsedUSD: 30, Percent: 33.3, WindowEnd: 1000},
			wantWeekly: &operationsWindow{LimitUSD: 240, UsedUSD: 100, Percent: 41.7, WindowEnd: 3000},
		},
		{
			name: "无 week 子限制回退主额度",
			main: mainUsage,
			subs: []model.SubscriptionSubQuotaUsage{hourUsage, dayUsage},
			want5h: &operationsWindow{LimitUSD: 90, UsedUSD: 30, Percent: 33.3, WindowEnd: 1000},
			wantWeekly: &operationsWindow{LimitUSD: 500, UsedUSD: 200, Percent: 40, WindowEnd: 4000},
		},
		{
			name: "day 不占任何槽位",
			main: nil,
			subs: []model.SubscriptionSubQuotaUsage{dayUsage},
			want5h: nil,
			wantWeekly: nil,
		},
		{
			name: "零值 limit 忽略",
			main: &model.MainQuotaUsage{LimitUSD: 0},
			subs: []model.SubscriptionSubQuotaUsage{{PeriodUnit: "hour", LimitUSD: 0, UsedUSD: 1}},
			want5h: nil,
			wantWeekly: nil,
		},
		{
			name: "全空",
			main: nil,
			subs: nil,
			want5h: nil,
			wantWeekly: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got5h, gotWeekly := pickOperationsWindows(tt.main, tt.subs)
			assert.Equal(t, tt.want5h, got5h)
			assert.Equal(t, tt.wantWeekly, gotWeekly)
		})
	}
}
