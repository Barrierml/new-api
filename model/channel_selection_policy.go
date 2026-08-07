package model

import "strings"

const ChannelTagSafe = "安全"

func (channel *Channel) IsExplicitlySafe() bool {
	return channel != nil && strings.TrimSpace(channel.GetTag()) == ChannelTagSafe
}

type ChannelSelectionPolicy struct {
	PricingLimit        ChannelPricingLimit
	AllowUnsafeChannels bool
}

type ChannelSelectionPolicyResult struct {
	Blocked       bool
	SafetyBlocked bool
	PricingResult ChannelPricingLimitResult
}

func (policy ChannelSelectionPolicy) WithPricingLimit(pricingLimit ChannelPricingLimit) ChannelSelectionPolicy {
	policy.PricingLimit = pricingLimit
	return policy
}

func (policy ChannelSelectionPolicy) Evaluate(channel *Channel) ChannelSelectionPolicyResult {
	if !policy.AllowUnsafeChannels && !channel.IsExplicitlySafe() {
		return ChannelSelectionPolicyResult{
			Blocked:       true,
			SafetyBlocked: true,
		}
	}

	pricingResult := policy.PricingLimit.Evaluate(channel.GetRatio())
	return ChannelSelectionPolicyResult{
		Blocked:       pricingResult.Blocked,
		PricingResult: pricingResult,
	}
}

type ChannelSafetyLimitError struct{}

func (e *ChannelSafetyLimitError) Error() string {
	return "no explicitly safe channel is available"
}
