package main

import (
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
