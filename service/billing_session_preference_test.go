package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newBillingPreferenceTestContext() *gin.Context {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	return ctx
}

func TestNewBillingSessionTokenPreferenceOverridesUserPreference(t *testing.T) {
	truncate(t)
	seedUser(t, 801, 1_000)
	relayInfo := &relaycommon.RelayInfo{
		RequestId:              "billing-override-request",
		UserId:                 801,
		IsPlayground:           true,
		OriginModelName:        "test-model",
		TokenBillingPreference: "wallet_only",
		UserSetting: dto.UserSetting{
			BillingPreference: "subscription_only",
		},
	}

	session, apiErr := NewBillingSession(newBillingPreferenceTestContext(), relayInfo, 10)
	require.Nil(t, apiErr)
	require.NotNil(t, session)
	assert.Equal(t, BillingSourceWallet, session.funding.Source())
	assert.Equal(t, "wallet_only", relayInfo.EffectiveBillingPreference)
	assert.Equal(t, "token", relayInfo.BillingPreferenceSource)
	require.NoError(t, session.Settle(10))

	quota, err := model.GetUserQuota(801, true)
	require.NoError(t, err)
	assert.Equal(t, 990, quota)
}

func TestNewBillingSessionEmptyTokenPreferenceUsesUserPreference(t *testing.T) {
	truncate(t)
	seedUser(t, 811, 1_000)
	relayInfo := &relaycommon.RelayInfo{
		RequestId:       "billing-inherit-request",
		UserId:          811,
		IsPlayground:    true,
		OriginModelName: "test-model",
		UserSetting: dto.UserSetting{
			BillingPreference: "wallet_only",
		},
	}

	session, apiErr := NewBillingSession(newBillingPreferenceTestContext(), relayInfo, 10)
	require.Nil(t, apiErr)
	require.NotNil(t, session)
	assert.Equal(t, BillingSourceWallet, session.funding.Source())
	assert.Equal(t, "wallet_only", relayInfo.EffectiveBillingPreference)
	assert.Equal(t, "user", relayInfo.BillingPreferenceSource)
	require.NoError(t, session.Settle(10))

	quota, err := model.GetUserQuota(811, true)
	require.NoError(t, err)
	assert.Equal(t, 990, quota)
}

func TestAppendBillingInfoRecordsEffectivePreference(t *testing.T) {
	relayInfo := &relaycommon.RelayInfo{
		BillingSource:              BillingSourceWallet,
		EffectiveBillingPreference: "wallet_only",
		BillingPreferenceSource:    "token",
	}
	other := map[string]interface{}{}

	appendBillingInfo(relayInfo, other)

	assert.Equal(t, BillingSourceWallet, other["billing_source"])
	assert.Equal(t, "wallet_only", other["billing_preference"])
	assert.Equal(t, "token", other["billing_preference_source"])
}
