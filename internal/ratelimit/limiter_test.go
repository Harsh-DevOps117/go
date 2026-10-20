package ratelimit

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestLocalWindowAndMemoryBound(t *testing.T) {
	l := New(2, time.Minute, "test", nil, nil)
	now := time.Now()
	for i := 0; i < 2; i++ {
		if ok, _ := l.local("client", now); !ok {
			t.Fatal("denied inside budget")
		}
	}
	if ok, retry := l.local("client", now); ok || retry != time.Minute {
		t.Fatal(ok, retry)
	}
	if ok, _ := l.local("client", now.Add(time.Minute)); !ok {
		t.Fatal("window did not reset")
	}
	for i := 0; i < 10000; i++ {
		l.local(fmt.Sprint(i), now.Add(time.Minute))
	}
	if len(l.entries) != 10000 {
		t.Fatalf("unbounded map: %d", len(l.entries))
	}
	if ok, _ := l.local("new-client", now.Add(time.Minute)); ok {
		t.Fatal("accepted beyond capacity")
	}
	if ok, _ := l.local("new-client", now.Add(2*time.Minute)); !ok {
		t.Fatal("expired entries were not released")
	}
}

func TestConcurrentLocalLimit(t *testing.T) {
	l := New(10, time.Minute, "test", nil, nil)
	var wg sync.WaitGroup
	var accepted atomic.Int32
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ok, _ := l.Allow(context.Background(), "same-ip"); ok {
				accepted.Add(1)
			}
		}()
	}
	wg.Wait()
	if accepted.Load() != 10 {
		t.Fatal(accepted.Load())
	}
}

func TestRedisOutageFallsBack(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "unused", MaxRetries: -1, Dialer: func(context.Context, string, string) (net.Conn, error) { return nil, fmt.Errorf("test outage") }})
	t.Cleanup(func() { _ = client.Close() })
	l := New(1, time.Minute, "test", client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if ok, _ := l.Allow(context.Background(), "client"); !ok {
		t.Fatal("outage rejected first local request")
	}
	if ok, _ := l.Allow(context.Background(), "client"); ok {
		t.Fatal("outage bypassed local limit")
	}
}

func TestSharedRedisLimitIntegration(t *testing.T) {
	raw := os.Getenv("TEST_REDIS_URL")
	if raw == "" {
		t.Skip("set TEST_REDIS_URL or run make test-integration")
	}
	opts, err := redis.ParseURL(raw)
	if err != nil {
		t.Fatal(err)
	}
	client := redis.NewClient(opts)
	t.Cleanup(func() { _ = client.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatal(err)
	}
	namespace := fmt.Sprintf("test-%d", time.Now().UnixNano())
	a, b := New(2, time.Second, namespace, client, nil), New(2, time.Second, namespace, client, nil)
	if ok, _ := a.Allow(ctx, "ip"); !ok {
		t.Fatal("first request denied")
	}
	if ok, _ := b.Allow(ctx, "ip"); !ok {
		t.Fatal("second request denied")
	}
	if ok, retry := b.Allow(ctx, "ip"); ok || retry <= 0 {
		t.Fatal("replicas did not share budget", ok, retry)
	}
	if a.degraded.Load() || b.degraded.Load() {
		t.Fatal("test silently fell back to local limits")
	}
}
