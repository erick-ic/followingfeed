package logger

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFromContextAddsRequestCorrelation(t *testing.T) {
	ctx := ContextWithFields(
		context.Background(),
		String("request_id", "request-1"),
	)

	base := &captureLogger{}
	FromContext(ctx, base).Info("message")

	assert.Equal(t, "request-1", base.fields["request_id"])
}

type captureLogger struct {
	fields map[string]any
}

func (*captureLogger) Debug(string, ...Field) {}
func (l *captureLogger) Info(_ string, fields ...Field) {
	for _, field := range fields {
		l.fields[field.Key] = field.Value
	}
}
func (*captureLogger) Warn(string, ...Field)  {}
func (*captureLogger) Error(string, ...Field) {}
func (l *captureLogger) With(fields ...Field) LoggerV1 {
	if l.fields == nil {
		l.fields = make(map[string]any)
	}
	for _, field := range fields {
		l.fields[field.Key] = field.Value
	}
	return l
}
