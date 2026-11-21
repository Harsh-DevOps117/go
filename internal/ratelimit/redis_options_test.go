package ratelimit

import (
	"reflect"
	"testing"

	"github.com/redis/go-redis/v9"
)

// TestRedisClientOptionsMatchSupportedRelease pins the go-redis release this
// service builds against. The dedicated pipeline pool and the client-side
// cache controls were added in v9.23.0, and the beta refresh-recency window
// was removed there. Reflection keeps the check runnable on older releases so
// the dependency upgrade itself is what makes it pass.
func TestRedisClientOptionsMatchSupportedRelease(t *testing.T) {
	options := reflect.TypeOf(redis.Options{})
	for _, name := range []string{
		"PipelinePoolSize",
		"ClientSideCacheRefreshOnInvalidate",
		"ClientSideCacheCoalesceMisses",
		"ClientSideCacheInvalidationBatchWindow",
	} {
		if _, ok := options.FieldByName(name); !ok {
			t.Fatalf("redis.Options.%s is missing; go-redis v9.23.0 or newer is required", name)
		}
	}
	if _, ok := options.FieldByName("ClientSideCacheRefreshRecencyWindow"); ok {
		t.Fatal("redis.Options.ClientSideCacheRefreshRecencyWindow was removed upstream")
	}
	if _, ok := reflect.TypeOf(redis.ClusterOptions{}).FieldByName("RouteByLatencyTolerance"); !ok {
		t.Fatal("redis.ClusterOptions.RouteByLatencyTolerance is missing; go-redis v9.23.0 or newer is required")
	}
}
