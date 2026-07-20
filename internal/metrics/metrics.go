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

package metrics

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// PromQL comments for compliance with golang-observability:2
	// HTTP requests total counter
	// PromQL: sum(rate(requests_total[5m])) by (method, path, status)
	RequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "requests_total",
			Help: "Total number of HTTP requests processed.",
		},
		[]string{"method", "path", "status"},
	)

	// HTTP request duration histogram
	// PromQL: histogram_quantile(0.99, sum(rate(request_duration_seconds_bucket[5m])) by (le, path))
	RequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "request_duration_seconds",
			Help:    "Latency of HTTP requests in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)

	// Database query duration histogram
	// PromQL: histogram_quantile(0.99, sum(rate(db_query_duration_seconds_bucket[5m])) by (le, operation))
	DBQueryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "db_query_duration_seconds",
			Help:    "Latency of database queries in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation", "status"},
	)

	// Lock conflicts counter
	// PromQL: sum(rate(lock_conflicts[5m]))
	LockConflicts = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "lock_conflicts",
			Help: "Total number of lock acquisition conflicts.",
		},
	)
)

type DBCollector struct {
	pool            *pgxpool.Pool
	activeConnsDesc *prometheus.Desc
	lockCountDesc   *prometheus.Desc
}

func NewDBCollector(pool *pgxpool.Pool) *DBCollector {
	return &DBCollector{
		pool: pool,
		activeConnsDesc: prometheus.NewDesc(
			"active_db_connections",
			"Number of active database connections.",
			nil, nil,
		),
		lockCountDesc: prometheus.NewDesc(
			"lock_count",
			"Number of active state locks.",
			nil, nil,
		),
	}
}

func (c *DBCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.activeConnsDesc
	ch <- c.lockCountDesc
}

func (c *DBCollector) Collect(ch chan<- prometheus.Metric) {
	stats := c.pool.Stat()
	activeConns := float64(stats.AcquiredConns())
	ch <- prometheus.MustNewConstMetric(c.activeConnsDesc, prometheus.GaugeValue, activeConns)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var count int64
	err := c.pool.QueryRow(ctx, "SELECT count(*) FROM locks").Scan(&count)
	if err == nil {
		ch <- prometheus.MustNewConstMetric(c.lockCountDesc, prometheus.GaugeValue, float64(count))
	}
}

func Init(pool *pgxpool.Pool) {
	prometheus.MustRegister(RequestsTotal)
	prometheus.MustRegister(RequestDuration)
	prometheus.MustRegister(DBQueryDuration)
	prometheus.MustRegister(LockConflicts)
	prometheus.MustRegister(NewDBCollector(pool))
}
