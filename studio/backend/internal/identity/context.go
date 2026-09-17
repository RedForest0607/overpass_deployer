package identity

import "context"

type ownerContextKey struct{}

func WithOwner(ctx context.Context, ownerID string) context.Context {
	return context.WithValue(ctx, ownerContextKey{}, ownerID)
}

func OwnerID(ctx context.Context) (string, bool) {
	ownerID, ok := ctx.Value(ownerContextKey{}).(string)
	return ownerID, ok && ownerID != ""
}
