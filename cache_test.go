package cache

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCache_NewGroup(t *testing.T) {
	config := &Config{
		Addr: "localhost:8080",
	}
	c := NewCache(config).(*cache)
	defer c.Close()

	getter := GetterFunc(func(ctx context.Context, key string, dest Sink) error {
		dest.Set(key, "value for "+key)
		return nil
	})

	group := c.NewGroup("testGroup", getter)
	assert.NotNil(t, group)
}
