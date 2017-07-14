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

// InitAuth should be called in the application's init(). It sets up the auth map.
func InitAuth() {
	auths.mu.Lock()
	auths.keys = make(map[string]auth)
	auths.mu.Unlock()
}

// SetAuth creates an auth entry with the given params
func SetAuth(key string, email string, expire time.Time) {
	auths.mu.Lock()
	defer auths.mu.Unlock()
	auths.keys[key] = auth{email: email, expire: expire}
}

// IsValidAuth ensures the the given key is valied and not expired
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

// PruneAuth removes expired keys from the auths.keys map
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
