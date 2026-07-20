// Copyright 2026 [Copyright Holder]
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// Author: [YOUR_NAME]

package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/sh0jitmy/pgstate-gateway/internal/config"
	"golang.org/x/time/rate"
)

type clientLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type IPRateLimiter struct {
	mu           sync.RWMutex
	clients      map[string]*clientLimiter
	mgr          *config.Manager
	currentLimit int
}

func NewIPRateLimiter(mgr *config.Manager) *IPRateLimiter {
	limiter := &IPRateLimiter{
		clients:      make(map[string]*clientLimiter),
		mgr:          mgr,
		currentLimit: mgr.Get().Security.RateLimit,
	}

	// Periodically clean up old IP entries to prevent memory leaks
	go limiter.cleanupLoop()

	return limiter
}

func (l *IPRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	for range ticker.C {
		l.mu.Lock()
		for ip, client := range l.clients {
			if time.Since(client.lastSeen) > 5*time.Minute {
				delete(l.clients, ip)
			}
		}
		l.mu.Unlock()
	}
}

func (l *IPRateLimiter) getLimiter(ip string, rateLimit int) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()

	// If rate limit has changed from configuration, update it
	if l.currentLimit != rateLimit {
		l.currentLimit = rateLimit
		for _, client := range l.clients {
			client.limiter.SetLimit(rate.Limit(float64(rateLimit) / 60.0))
			client.limiter.SetBurst(rateLimit)
		}
	}

	client, exists := l.clients[ip]
	if !exists {
		// e.g. 100 requests per minute -> 100/60 per second, with burst of 100
		lim := rate.NewLimiter(rate.Limit(float64(rateLimit)/60.0), rateLimit)
		client = &clientLimiter{
			limiter:  lim,
			lastSeen: time.Now(),
		}
		l.clients[ip] = client
	} else {
		client.lastSeen = time.Now()
	}

	return client.limiter
}

func getIP(r *http.Request) string {
	// Check X-Forwarded-For first
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}

	// Check X-Real-IP
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fallback to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

func (l *IPRateLimiter) Handler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cfg := l.mgr.Get()
			ip := getIP(r)

			limiter := l.getLimiter(ip, cfg.Security.RateLimit)
			if !limiter.Allow() {
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
