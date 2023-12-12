package v1alpha1

import "context"

type contextKey struct{}

// GovernorEventCorrelationIDContextKey is the context key for the correlation ID
var governorEventCorrelationIDContextKey = &contextKey{}

// InjectCorrelationID injects the correlation ID into the context
func InjectCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, governorEventCorrelationIDContextKey, correlationID)
}

// ExtractCorrelationID extracts the correlation ID from the context
func ExtractCorrelationID(ctx context.Context) string {
	if cid, ok := ctx.Value(governorEventCorrelationIDContextKey).(string); ok {
		return cid
	}

	return ""
}
