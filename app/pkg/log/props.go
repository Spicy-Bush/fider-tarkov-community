package log

import (
	"context"
	"sync"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
)

const (
	// PropertyKeySessionID is the session id of current logger
	PropertyKeySessionID = "SessionID"
	// PropertyKeyContextID is the context id of current logger
	PropertyKeyContextID = "ContextID"
	// PropertyKeyUserID is the user id of current logger
	PropertyKeyUserID = "UserID"
	// PropertyKeyTenantID is the tenant id of current logger
	PropertyKeyTenantID = "TenantID"
	// PropertyKeyTag is the tag of current logger
	PropertyKeyTag = "Tag"
)

func GetProperties(ctx context.Context) dto.Props {
	sm, ok := ctx.Value(app.LogPropsCtxKey).(*sync.Map)
	if !ok {
		return dto.Props{}
	}
	result := dto.Props{}
	sm.Range(func(k, v any) bool {
		result[k.(string)] = v
		return true
	})
	return result
}

func GetProperty(ctx context.Context, key string) any {
	sm, ok := ctx.Value(app.LogPropsCtxKey).(*sync.Map)
	if ok {
		v, _ := sm.Load(key)
		return v
	}
	return nil
}

func WithProperty(ctx context.Context, key string, value any) context.Context {
	return WithProperties(ctx, dto.Props{
		key: value,
	})
}

func WithProperties(ctx context.Context, kv dto.Props) context.Context {
	sm, ok := ctx.Value(app.LogPropsCtxKey).(*sync.Map)
	if !ok {
		sm = &sync.Map{}
		ctx = context.WithValue(ctx, app.LogPropsCtxKey, sm)
	}

	for key, value := range kv {
		sm.Store(key, value)
	}

	return ctx
}
