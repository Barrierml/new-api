package model

import (
	"fmt"
	"math"

	"github.com/shopspring/decimal"
)

// ChannelPricingLimit is evaluated before channel priority and weight selection.
// Input prices are expressed in USD per 1M text input tokens.
type ChannelPricingLimit struct {
	MaxChannelRatio      float64
	MaxInputPrice        float64
	BaseInputPrice       float64
	GroupRatio           float64
	InputPriceComparable bool
	ratioOnly            bool
}

type ChannelPricingLimitResult struct {
	Blocked              bool
	ChannelRatioExceeded bool
	InputPriceExceeded   bool
	InputPriceComparable bool
	ChannelRatio         float64
	ActualInputPrice     float64
}

func (limit ChannelPricingLimit) WithGroupRatio(groupRatio float64) ChannelPricingLimit {
	limit.GroupRatio = groupRatio
	if groupRatio < 0 || math.IsNaN(groupRatio) || math.IsInf(groupRatio, 0) {
		limit.InputPriceComparable = false
	}
	return limit
}

func (limit ChannelPricingLimit) Evaluate(channelRatio float64) ChannelPricingLimitResult {
	result := ChannelPricingLimitResult{
		ChannelRatio:         channelRatio,
		InputPriceComparable: limit.InputPriceComparable,
	}
	result.ChannelRatioExceeded = limit.MaxChannelRatio > 0 && channelRatio > limit.MaxChannelRatio
	if !result.ChannelRatioExceeded {
		return result
	}

	if limit.ratioOnly {
		result.Blocked = true
		return result
	}
	if limit.MaxInputPrice == 0 {
		return result
	}
	if !limit.InputPriceComparable {
		result.Blocked = true
		return result
	}
	if limit.MaxInputPrice < 0 || math.IsNaN(limit.MaxInputPrice) || math.IsInf(limit.MaxInputPrice, 0) ||
		limit.BaseInputPrice < 0 || math.IsNaN(limit.BaseInputPrice) || math.IsInf(limit.BaseInputPrice, 0) {
		result.InputPriceComparable = false
		result.Blocked = true
		return result
	}

	actualInputPrice := decimal.NewFromFloat(limit.BaseInputPrice).
		Mul(decimal.NewFromFloat(limit.GroupRatio)).
		Mul(decimal.NewFromFloat(channelRatio))
	result.ActualInputPrice = actualInputPrice.InexactFloat64()
	result.InputPriceExceeded = actualInputPrice.GreaterThan(decimal.NewFromFloat(limit.MaxInputPrice))
	result.Blocked = result.InputPriceExceeded
	return result
}

type ChannelPricingLimitError struct {
	MaxChannelRatio      float64
	MaxInputPrice        float64
	MinAvailableRatio    float64
	MinActualInputPrice  float64
	InputPriceComparable bool
}

func NewChannelPricingLimitError(limit ChannelPricingLimit, result ChannelPricingLimitResult) *ChannelPricingLimitError {
	return &ChannelPricingLimitError{
		MaxChannelRatio:      limit.MaxChannelRatio,
		MaxInputPrice:        limit.MaxInputPrice,
		MinAvailableRatio:    result.ChannelRatio,
		MinActualInputPrice:  result.ActualInputPrice,
		InputPriceComparable: result.InputPriceComparable,
	}
}

func (e *ChannelPricingLimitError) Consider(result ChannelPricingLimitResult) {
	if e == nil {
		return
	}
	if e.InputPriceComparable && result.InputPriceComparable {
		if result.ActualInputPrice < e.MinActualInputPrice ||
			(result.ActualInputPrice == e.MinActualInputPrice && result.ChannelRatio < e.MinAvailableRatio) {
			e.MinActualInputPrice = result.ActualInputPrice
			e.MinAvailableRatio = result.ChannelRatio
		}
		return
	}
	e.InputPriceComparable = false
	if result.ChannelRatio < e.MinAvailableRatio {
		e.MinAvailableRatio = result.ChannelRatio
	}
}

func (e *ChannelPricingLimitError) Merge(other *ChannelPricingLimitError) {
	if e == nil || other == nil {
		return
	}
	e.Consider(ChannelPricingLimitResult{
		InputPriceComparable: other.InputPriceComparable,
		ChannelRatio:         other.MinAvailableRatio,
		ActualInputPrice:     other.MinActualInputPrice,
	})
}

func (e *ChannelPricingLimitError) Error() string {
	if e.InputPriceComparable {
		return fmt.Sprintf(
			"no channel within pricing limits: max channel ratio %.4g, max input price %.4g; lowest rejected input price is %.4g",
			e.MaxChannelRatio, e.MaxInputPrice, e.MinActualInputPrice,
		)
	}
	return fmt.Sprintf("no channel within ratio limit %.4g; minimum available ratio is %.4g", e.MaxChannelRatio, e.MinAvailableRatio)
}
