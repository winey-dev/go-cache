package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGroup_GetAndSet(t *testing.T) {
	getter := GetterFunc(func(ctx context.Context, key string, dest Sink) error {
		dest.Set(key, "value for "+key)
		return nil
	})

	group := newGroup("testGroup", getter, time.Minute, nil)

	// Test getting a value via GetterFunc
	val, err := group.Get(context.Background(), "missingKey")
	assert.NoError(t, err)
	assert.Equal(t, "value for missingKey", val)
}

func TestGroup_TTLExpiration(t *testing.T) {
	var cnt int = 0
	getter := GetterFunc(func(ctx context.Context, key string, dest Sink) error {
		if cnt == 0 {
			// only set the value once
			dest.Set(key, "value for "+key)
		}
		cnt += 1
		return nil
	})
	group := newGroup("testGroup", getter, time.Millisecond*100, nil)
	group.Get(context.Background(), "testKey")
	time.Sleep(time.Millisecond * 150)
	val, err := group.Get(context.Background(), "testKey")
	assert.Error(t, err)
	assert.Nil(t, val)
}

func TestGroup_Delete(t *testing.T) {
	var cnt int = 0
	getter := GetterFunc(func(ctx context.Context, key string, dest Sink) error {
		if cnt == 0 {
			// only set the value once
			dest.Set(key, "value for "+key)
		}
		cnt += 1
		return nil
	})

	deleteChan := make(chan deleteEvent, 1)
	group := newGroup("testGroup", getter, time.Minute, deleteChan)

	group.Get(context.Background(), "testKey")

	group.Del("testKey")

	val, err := group.Get(context.Background(), "testKey")
	assert.Error(t, err)
	assert.Nil(t, val)

	// Verify delete event is sent
	select {
	case event := <-deleteChan:
		assert.Equal(t, "testGroup", event.group)
		assert.Equal(t, "testKey", event.key)
	default:
		t.Fatal("expected delete event")
	}
}
