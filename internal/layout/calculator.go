// Package layout provides struct layout calculation and optimization.
package layout

import (
	"go/types"

	itypes "github.com/flaticols/pleasehitcache/internal/types"
	"golang.org/x/tools/go/analysis"
)

// Calculator calculates struct layout information.
type Calculator struct {
	pass          *analysis.Pass
	cacheLineSize int64
}

// New creates a new Calculator.
func New(pass *analysis.Pass, cacheLineSize int64) *Calculator {
	return &Calculator{
		pass:          pass,
		cacheLineSize: cacheLineSize,
	}
}

// Calculate computes the layout for a struct.
func (c *Calculator) Calculate(info *itypes.StructInfo) {
	sizes := c.pass.TypesSizes

	info.Size = sizes.Sizeof(info.StructType)
	info.Alignment = sizes.Alignof(info.StructType)

	numFields := info.StructType.NumFields()
	if numFields == 0 {
		return
	}

	fields := make([]*types.Var, numFields)
	for i := range numFields {
		fields[i] = info.StructType.Field(i)
	}
	offsets := sizes.Offsetsof(fields)

	info.Fields = make([]itypes.FieldInfo, numFields)
	for i, field := range fields {
		info.Fields[i] = itypes.FieldInfo{
			Name:      field.Name(),
			Type:      field.Type().String(),
			Offset:    offsets[i],
			Size:      sizes.Sizeof(field.Type()),
			Alignment: sizes.Alignof(field.Type()),
		}
	}
}

// CacheLineSize returns the configured cache line size.
func (c *Calculator) CacheLineSize() int64 {
	return c.cacheLineSize
}
