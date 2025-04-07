# go-cache

go-cache is a simple and efficient caching library that stores data in memory and supports TTL (Time-To-Live). It works in distributed environments and provides HTTP endpoints for managing cache data.

## Installation

To install go-cache, run the following command:

```bash
go get github.com/winey-dev/go-cache
```

## Usage

### 1. Basic Configuration and Cache Creation

```go
import (
	"context"
	"github.com/winey-dev/go-cache"
)

func main() {
	config := &cache.Config{
		Addr: "localhost:8080",
		CacheCleanupIntervalSec: 10,
	}

	c := cache.NewCache(config)
	defer c.Close()
}
```

### 2. Creating Groups and Managing Data

```go
getter := cache.GetterFunc(func(ctx context.Context, key string, dest cache.Sink) error {
	dest.Set(key, "Value for " + key)
	return nil
})

group := c.NewGroup("exampleGroup", getter)

// Retrieving data
val, err := group.Get(context.Background(), "exampleKey")
if err != nil {
	fmt.Println("Error:", err)
} else {
	fmt.Println("Value:", val)
}

// Deleting data
group.Del("exampleKey")
```

### 3. Using the HTTP Server

go-cache provides an HTTP server to manage cache data.

```go
config := &cache.Config{
	Addr: "localhost:8080",
}

c := cache.NewCache(config)
defer c.Close()

// The HTTP server starts automatically.
```

HTTP Endpoints:
- `GET /{groupName}/{key}`: Retrieve the value of a specific key.
- `DELETE /{groupName}/{key}`: Delete a specific key.

### 4. Setting TTL (Time-To-Live)

```go
group := c.NewGroupWithTTL("exampleGroup", getter, time.Minute*5)
```

### 5. Multi-Node Cache Example

go-cache supports a multi-node setup where changes in one node are propagated to peers. When data is deleted in one node, the peer nodes will fetch the updated data using the `GetterFunc`.

#### Example Setup

Assume we have three nodes: `a`, `b`, and `c`. Node `a` modifies a key, and the change is propagated to `b` and `c`.

```go
package main

import (
	"context"
	"fmt"
	"github.com/winey-dev/go-cache"
)

func main() {
	// Node A configuration
	nodeAConfig := &cache.Config{
		Addr: "localhost:8080",
		PeerAddresses: []string{"localhost:8081", "localhost:8082"},
		CacheCleanupIntervalSec: 10,
	}
	nodeA := cache.NewCache(nodeAConfig)
	defer nodeA.Close()

	// Define GetterFunc
	// Implement the logic to fetch the actual value for the given key and set it in the destination sink.
	getter := cache.GetterFunc(func(ctx context.Context, key string, dest cache.Sink) error {
		fmt.Printf("Fetching data for key: %s\n", key)
		dest.SetString(fmt.Sprintf("Value for %s", key))
		return nil
	})

	// Create a group in Node A
	groupA := nodeA.NewGroup("exampleGroup", getter)

	// Simulate a key deletion in Node A
	groupA.Del("exampleKey")

	// Node B and Node C will fetch the updated data via GetterFunc
	// when they attempt to access the deleted key.
}
```

#### How It Works

1. Node `a` deletes a key using `group.Del()`.
2. The delete event is propagated to peers (`b` and `c`) via the `DELETE` API.
3. When `b` or `c` tries to access the deleted key, they invoke the `GetterFunc` to fetch the updated data.

This ensures that all nodes in the cluster have consistent and up-to-date data.

## Contributing

If you would like to contribute, please fork this repository and create a Pull Request.

## License

This project is distributed under the [Apache 2.0 License](LICENSE).
