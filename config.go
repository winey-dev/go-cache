package cache

type Config struct {
	// localhost:8080
	Addr string
	// localhost:8081, localhost:8082
	PeerAddresses []string //

	// service-headless.namespace
	HeadlessServiceName string //

	// 4567
	HeadlessServicePort int //

	CacheCleanupIntervalSec         int
	HeadlessServiceWatchIntervalSec int
}
