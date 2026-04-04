package provider

import (
	"context"
	"fm-my-canvas/types"
)

type Provider interface {
	Stream(ctx context.Context, messages []types.Message, cb func(chunk string)) error
}
