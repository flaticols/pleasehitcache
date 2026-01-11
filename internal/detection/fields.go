package detection

import (
	"go/types"
)

// mutexTypes are types that indicate concurrent access patterns.
var mutexTypes = map[string]bool{
	"sync.Mutex":   true,
	"sync.RWMutex": true,
}

// atomicTypes are atomic types from sync/atomic.
var atomicTypes = map[string]bool{
	"sync/atomic.Int32":   true,
	"sync/atomic.Int64":   true,
	"sync/atomic.Uint32":  true,
	"sync/atomic.Uint64":  true,
	"sync/atomic.Bool":    true,
	"sync/atomic.Pointer": true,
	"sync/atomic.Value":   true,
	"atomic.Int32":        true,
	"atomic.Int64":        true,
	"atomic.Uint32":       true,
	"atomic.Uint64":       true,
	"atomic.Bool":         true,
	"atomic.Pointer":      true,
	"atomic.Value":        true,
}

// HasMutexField checks if a struct has sync.Mutex or sync.RWMutex fields.
func HasMutexField(s *types.Struct) bool {
	for i := range s.NumFields() {
		field := s.Field(i)
		typeName := field.Type().String()
		if mutexTypes[typeName] {
			return true
		}
	}
	return false
}

// HasAtomicField checks if a struct has atomic.* typed fields.
func HasAtomicField(s *types.Struct) bool {
	for i := range s.NumFields() {
		field := s.Field(i)
		typeName := field.Type().String()
		if atomicTypes[typeName] {
			return true
		}
		// Check for pointer to atomic types
		if ptr, ok := field.Type().(*types.Pointer); ok {
			if atomicTypes[ptr.Elem().String()] {
				return true
			}
		}
	}
	return false
}

// GetMutexFields returns the names of mutex fields in a struct.
func GetMutexFields(s *types.Struct) []string {
	var fields []string
	for i := range s.NumFields() {
		field := s.Field(i)
		typeName := field.Type().String()
		if mutexTypes[typeName] {
			fields = append(fields, field.Name())
		}
	}
	return fields
}

// GetAtomicFields returns the names of atomic fields in a struct.
func GetAtomicFields(s *types.Struct) []string {
	var fields []string
	for i := range s.NumFields() {
		field := s.Field(i)
		typeName := field.Type().String()
		if atomicTypes[typeName] {
			fields = append(fields, field.Name())
		}
	}
	return fields
}
