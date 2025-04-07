package main

import (
	"context"
	"fmt"

	"github.com/winey-dev/go-cache"
)

func main() {
	// Config 설정
	config := &cache.Config{
		Addr:                            "localhost:8080",
		CacheCleanupIntervalSec:         10,
		HeadlessServiceWatchIntervalSec: 15,
	}

	// Cache 생성
	c := cache.NewCache(config)

	// Getter 정의
	getter := cache.GetterFunc(func(ctx context.Context, key string, dest cache.Sink) error {
		fmt.Printf("Fetching key: %s\n", key)
		dest.Set(key, fmt.Sprintf("Value for %s", key))
		return nil
	})

	// Group 생성
	group := c.NewGroup("exampleGroup", getter)

	// Key-Value 가져오기
	val, err := group.Get(context.Background(), "exampleKey")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Value: %v\n", val)
	}

	// Key 삭제
	group.Del("exampleKey")

	// Key 삭제 후 가져오기
	val, err = group.Get(context.Background(), "exampleKey")
	if err != nil {
		fmt.Printf("Error after deletion: %v\n", err)
	} else {
		fmt.Printf("Value after deletion: %v\n", val)
	}

	// Cache 종료
	c.Close()
}
