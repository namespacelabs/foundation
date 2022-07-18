package logging

import "context"

type contextKey string

var ck contextKey = "ns.ctx.request-id"

type requestData struct {
	rid string
}

func RequestID(ctx context.Context) string {
	v := ctx.Value(ck)
	if v != nil {
		return v.(requestData).rid
	}

	return ""
}

func withRequestID(ctx context.Context, rd requestData) context.Context {
	return context.WithValue(ctx, ck, rd)
}
