package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

type Sink interface {
	Set(key string, val any)
}

type data struct {
	val     any
	ttlTime time.Time
}

type Group interface {
	Get(ctx context.Context, key string) (any, error)
	Del(key string)
}

type group struct {
	mtx        sync.RWMutex
	name       string
	data       map[string]data
	getter     Getter
	defttl     time.Duration
	deleteChan chan deleteEvent
}

func newGroup(name string, getter Getter, defttl time.Duration, deleteChan chan deleteEvent) *group {
	return &group{
		name:       name,
		data:       make(map[string]data),
		defttl:     defttl,
		getter:     getter,
		deleteChan: deleteChan,
	}
}

func (g *group) get(ctx context.Context, key string) (any, error) {
	g.mtx.RLock()
	data, hit := g.data[key]
	g.mtx.RUnlock()
	if !hit {
		return nil, fmt.Errorf("%s not found", key)
	}
	// Check if the data is expired
	//ttltime := 15초 time now 20초
	now := time.Now()
	if now.After(data.ttlTime) {
		g.mtx.Lock()
		delete(g.data, key)
		g.mtx.Unlock()
		return nil, errors.New("cache expired")
	}

	g.mtx.Lock()
	data.ttlTime = time.Now().Add(g.defttl)
	g.data[key] = data
	g.mtx.Unlock()

	return data.val, nil
}

func (g *group) Get(ctx context.Context, key string) (any, error) {
	if val, err := g.get(ctx, key); err == nil {
		return val, nil
	}
	if err := g.getter.Get(ctx, key, g); err != nil {
		return nil, err
	}
	return g.get(ctx, key)
}

// Sink
func (g *group) Set(key string, val any) {
	data := data{
		val:     val,
		ttlTime: time.Now().Add(g.defttl),
	}
	g.mtx.Lock()
	g.data[key] = data
	g.mtx.Unlock()
}

func (g *group) Del(key string) {
	g.mtx.Lock()
	delete(g.data, key)
	g.mtx.Unlock()

	g.deleteChan <- deleteEvent{group: g.name, key: key}
	// cache peer send delete
}

func (g *group) ttlCleanUp(now time.Time) {
	g.mtx.Lock()
	defer g.mtx.Unlock()

	for key, val := range g.data {
		if now.After(val.ttlTime) {
			delete(g.data, key)
		}
	}
}

func (g *group) JSONMarshal() ([]byte, error) {
	return json.Marshal(g.data)
}

func (g *group) JSONMarshalIndent(prefix, indent string) ([]byte, error) {
	return json.MarshalIndent(g.data, prefix, indent)
}
