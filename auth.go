package main

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"
)

type auth struct {
	email  string
	expire time.Time
}

var auths struct {
	mu   sync.Mutex
	keys map[string]auth
}

func InitAuth() {
	auths.mu.Lock()
	auths.keys = make(map[string]auth)
	auths.mu.Unlock()
}

func SetAuth(key string, email string, expire time.Time) {
	auths.mu.Lock()
	defer auths.mu.Unlock()
	auths.keys[key] = auth{email: email, expire: expire}
}

func IsValidAuth(key string) (string, bool) {
	auths.mu.Lock()
	defer auths.mu.Unlock()
	v, ok := auths.keys[key]
	if !ok {
		return "", false
	}
	if v.expire.Unix() < time.Now().Unix() {
		return "", false
	}
	return v.email, true
}

func PruneAuth() {
	auths.mu.Lock()
	defer auths.mu.Unlock()
	keys := make(map[string]auth)
	now := time.Now().Unix()
	for k, v := range auths.keys {
		if v.expire.Unix() > now {
			keys[k] = v
		}
	}
	auths.keys = keys
}

func AuthMW(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("auth")
		if err != nil {
			http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
			return
		}
		var email string
		var ok bool
		if email, ok = IsValidAuth(cookie.Value); !ok {
			log.Println("bad cookie. Tampering?")
			http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
			return
		}
		if email == "" {
			log.Println("something has gone horribly wrong")
		}
		ctx := r.Context()
		ctx = context.WithValue(ctx, ctxEmail, email)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
	return http.HandlerFunc(fn)
}
