package cache

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"slices"
	"sync"
	"time"
)

const (
	defttl                              = time.Hour
	defaultCacheClearInterval           = time.Duration(0) // infinite
	defaultHeadlessServiceWatchInterval = time.Second
)

type deleteEvent struct {
	group string
	key   string
}

type cache struct {
	// Peer 목록

	addr string

	peerAddresses []string

	// Headless Service
	headlessServiceName string
	headlessServicePort int

	// 동기화
	mtx sync.RWMutex

	// gorutine 제어
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc

	// ttl 이 지난 cache 삭제 주기
	ttlCleanupInterval time.Duration

	// headless service 목록에서 peer 변경 감지를 확인하는 주기
	headlessServiceWatchInterval time.Duration

	// group data
	group map[string]*group

	httpServ *http.Server

	deleteChan chan deleteEvent
}

type Getter interface {
	Get(ctx context.Context, key string, data Sink) error
}

type GetterFunc func(ctx context.Context, key string, data Sink) error

func (f GetterFunc) Get(ctx context.Context, key string, dest Sink) error {
	return f(ctx, key, dest)
}

type Cache interface {
	NewGroup(name string, getter Getter) Group
	NewGroupWithTTL(name string, getter Getter, ttl time.Duration) Group
	GetGroup(name string) Group
	Close()
}

func NewCache(config *Config) Cache {
	cache := new(cache)
	cache.group = make(map[string]*group)
	cache.ctx, cache.cancel = context.WithCancel(context.Background())

	if config.CacheCleanupIntervalSec <= 0 {
		cache.ttlCleanupInterval = defaultCacheClearInterval
	} else {
		cache.ttlCleanupInterval = time.Duration(config.CacheCleanupIntervalSec) * time.Second
	}

	if config.HeadlessServiceWatchIntervalSec <= 0 {
		cache.headlessServiceWatchInterval = defaultHeadlessServiceWatchInterval
	} else {
		cache.headlessServiceWatchInterval = time.Duration(config.HeadlessServiceWatchIntervalSec) * time.Second
	}

	cache.headlessServiceName = config.HeadlessServiceName

	if config.HeadlessServicePort < 4000 {
		cache.headlessServicePort = 4567
	} else {
		cache.headlessServicePort = config.HeadlessServicePort
	}

	if cache.ttlCleanupInterval != 0 {
		cache.wg.Add(1)
		go cache.ttlCleanUp()
	}

	if cache.headlessServiceName != "" {
		cache.wg.Add(1)
		go cache.watchHeadlessService()
		cache.addr = fmt.Sprintf(":%d", cache.headlessServicePort)
		cache.newHTTPServer(cache.addr)
	} else if len(config.PeerAddresses) != 0 && config.Addr != "" {
		// peerAddresses 목록에
		for _, peer := range config.PeerAddresses {
			if peer != config.Addr {
				cache.peerAddresses = append(cache.peerAddresses, peer)
			}
		}
		cache.addr = config.Addr
		cache.newHTTPServer(cache.addr)
	}

	if cache.httpServ != nil {
		cache.deleteChan = make(chan deleteEvent)
		cache.wg.Add(1)
		go cache.deleteEventWorker()
		cache.wg.Add(1)
		go cache.startHTTPServer()
	}

	return cache
}

func (c *cache) NewGroup(name string, getter Getter) Group {
	return c.NewGroupWithTTL(name, getter, defttl)
}

func (c *cache) NewGroupWithTTL(name string, getter Getter, ttl time.Duration) Group {
	group := newGroup(name, getter, ttl, c.deleteChan)
	c.mtx.Lock()
	c.group[name] = group
	c.mtx.Unlock()
	return group
}

func (c *cache) GetGroup(name string) Group {
	c.mtx.RLock()
	g, ok := c.group[name]
	if !ok {
		c.mtx.RUnlock()
		return nil
	}
	c.mtx.RUnlock()
	return g
}

func (c *cache) getGroupByName(name string) (*group, error) {
	g := c.GetGroup(name)
	if g == nil {
		return nil, fmt.Errorf("group '%s' not found", name)
	}
	return g.(*group), nil
}

func (c *cache) ttlCleanUp() {
	defer c.wg.Done()

	ticker := time.NewTicker(c.ttlCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			now := time.Now()
			c.mtx.Lock()
			for _, group := range c.group {
				group.ttlCleanUp(now)
			}
			c.mtx.Unlock()
		case <-c.ctx.Done():
			return
		}
	}
}

func (c *cache) watchHeadlessService() {
	defer c.wg.Done()

	ticker := time.NewTicker(c.headlessServiceWatchInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			newPeers := c.getCurrentPeers() // 최신 peers 조회
			c.mtx.Lock()                    // 동기화

			// 삭제된 노드 확인
			for _, oldPeer := range c.peerAddresses {
				found := slices.Contains(newPeers, oldPeer)
				if !found {
					fmt.Printf("node %s has been removed.\n", oldPeer)
				}
			}

			// 추가된 노드 확인
			for _, newPeer := range newPeers {
				found := slices.Contains(c.peerAddresses, newPeer)
				if !found {
					fmt.Printf("node %s has been added.\n", newPeer)
				}
			}

			// c.peerAddresses를 newPeers로 업데이트
			c.peerAddresses = newPeers
			c.mtx.Unlock()
		case <-c.ctx.Done():
			return
		}
	}
}

func getLocalIPs() map[string]struct{} {
	localIPs := make(map[string]struct{})

	interfaces, err := net.Interfaces()
	if err != nil {
		return localIPs
	}

	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil || ip.IsLoopback() {
				continue
			}

			localIPs[ip.String()] = struct{}{} // 로컬 IP 저장
		}
	}
	return localIPs
}

func (c *cache) getCurrentPeers() []string {
	addrs, err := net.LookupHost(c.headlessServiceName)
	if err != nil {
		return nil
	}

	localIPs := getLocalIPs() // 현재 노드의 IP 목록 가져오기
	peers := make([]string, 0, len(addrs))

	for _, addr := range addrs {
		if _, exists := localIPs[addr]; exists {
			continue // 현재 노드의 IP는 제외
		}
		peers = append(peers, fmt.Sprintf("%s:%d", addr, c.headlessServicePort))
	}
	return peers
}

func (c *cache) startHTTPServer() {
	defer c.wg.Done()
	if err := c.httpServ.ListenAndServe(); err != nil {
		panic(err)
	}
}

func (c *cache) deleteEventWorker() {
	for {
		select {
		case event, ok := <-c.deleteChan:
			if !ok {
				return
			}
			c.propagateDelete(event.group, event.key)
		case <-c.ctx.Done():
			return
		}
	}
}

func (c *cache) propagateDelete(group, key string) {
	for _, peer := range c.peerAddresses {
		url := fmt.Sprintf("http://%s/%s/%s", peer, group, key)
		req, err := http.NewRequest("DELETE", url, nil)
		if err != nil {
			continue
		}
		client := &http.Client{Timeout: 2 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()
	}
}

func (c *cache) Close() {
	c.cancel()
	if c.httpServ != nil {
		close(c.deleteChan)
		c.httpServ.Shutdown(c.ctx)
		c.httpServ.Close()
	}
	c.wg.Wait()
}
