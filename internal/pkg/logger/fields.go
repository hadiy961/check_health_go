package logger

import "go.uber.org/zap"

// Uint64 creates a field with a uint64 value
func Uint64(key string, value uint64) zap.Field {
	return zap.Uint64(key, value)
}

// We can keep the Field type for backward compatibility,
// but we won't use it for the logger functions
type Field struct {
	Key   string
	Value interface{}
}
