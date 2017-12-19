package adaptr

import (
	"net/http"
	"context"
	"sync"
	xCtx "golang.org/x/net/context"
	)

type Adapter func(handle http.Handler) http.Handler

func Adapt(h http.Handler, adapters ...Adapter) http.Handler {
	for i := len(adapters) - 1; i >= 0; i-- {
		h = adapters[i](h)
	}
	return h
}
type NewPlatformCtx func(*http.Request)context.Context
type NewPlatformXCtx func(*http.Request)xCtx.Context

func PlatformXCtxAdapter(NewContextFn NewPlatformXCtx ) Adapter {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h.ServeHTTP(w, r.WithContext(NewContextFn(r)))
		})
	}
}

func PlatformCtxAdapter(NewContextFn NewPlatformCtx ) Adapter {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h.ServeHTTP(w, r.WithContext(NewContextFn(r)))
		})
	}
}

func SetCtxValue(r *http.Request, key, value interface{}) (*http.Request) {
	return r.WithContext(context.WithValue(r.Context(), key, value))
}

func GetCtxValue(r *http.Request, key interface{}) interface{} {
	return r.Context().Value(key)
}

func GetCtxValueStr(r *http.Request, key interface{}) string {
	return GetCtxValue(r, key).(string)
}

func CallOnce(f func(w http.ResponseWriter, r *http.Request)) Adapter {
	once:=sync.Once{}
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			once.Do(func() {
				f(w, r)
			})
			h.ServeHTTP(w, r)
		})
	}
}

func JsonContentType() Adapter {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			h.ServeHTTP(w, r)
		})
	}
}