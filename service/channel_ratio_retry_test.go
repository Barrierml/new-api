package service

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
)

// After a retry the relay switches channels and refreshes
// ChannelMeta.ChannelRatio, while PriceData.ChannelRatio (computed once before
// the retry loop) stays at the first pick. Billing must charge for the channel
// that actually served the request.
func TestEffectiveChannelRatio(t *testing.T) {
	cases := []struct {
		name       string
		metaRatio  float64
		hasMeta    bool
		priceRatio float64
		want       float64
	}{
		{"retry to pricier channel uses served ratio", 23.1, true, 1.31, 23.1},
		{"no retry meta==price", 1.31, true, 1.31, 1.31},
		{"nil meta falls back to price", 0, false, 6.5562, 6.5562},
		{"zero meta falls back to price", 0, true, 4.2, 4.2},
		{"all zero defaults to 1", 0, false, 0, 1.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			info := &relaycommon.RelayInfo{}
			info.PriceData = types.PriceData{ChannelRatio: tc.priceRatio}
			if tc.hasMeta {
				info.ChannelMeta = &relaycommon.ChannelMeta{ChannelRatio: tc.metaRatio}
			}
			if got := effectiveChannelRatio(info); got != tc.want {
				t.Fatalf("effectiveChannelRatio = %v, want %v", got, tc.want)
			}
		})
	}
	if got := effectiveChannelRatio(nil); got != 1.0 {
		t.Fatalf("effectiveChannelRatio(nil) = %v, want 1.0", got)
	}
}
