package layout

import (
	itypes "github.com/flaticols/pleasehitcache/internal/types"
)

// CalculatePadding determines padding needed for cache line alignment.
func (c *Calculator) CalculatePadding(info *itypes.StructInfo) int64 {
	if info.Size >= c.cacheLineSize {
		return 0
	}
	return c.cacheLineSize - info.Size
}

// FitsInCacheLine checks if a struct can fit in a single cache line with padding.
func (c *Calculator) FitsInCacheLine(info *itypes.StructInfo) bool {
	return info.Size <= c.cacheLineSize
}

// ExceedsCacheLine checks if a struct is larger than the cache line.
func (c *Calculator) ExceedsCacheLine(info *itypes.StructInfo) bool {
	return info.Size > c.cacheLineSize
}

// PaddingRatio returns the ratio of padding to original size.
func (c *Calculator) PaddingRatio(info *itypes.StructInfo) float64 {
	if info.Size == 0 {
		return 0
	}
	padding := c.CalculatePadding(info)
	return float64(padding) / float64(info.Size)
}

// IsPaddingReasonable checks if padding wouldn't more than double the struct size.
func (c *Calculator) IsPaddingReasonable(info *itypes.StructInfo) bool {
	return c.PaddingRatio(info) <= 1.0
}
