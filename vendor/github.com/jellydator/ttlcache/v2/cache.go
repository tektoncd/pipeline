package ttlcache

import (
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// CheckExpireCallback is used as a callback for an external check on item expiration
type CheckExpireCallback func(key string, value interface{}) bool

// ExpireCallback is used as a callback on item expiration or when notifying of an item new to the cache
// Note that ExpireReasonCallback will be the successor of this function in the next major release.
type ExpireCallback func(key string, value interface{})

// ExpireReasonCallback is used as a callback on item expiration with extra information why the item expired.
type ExpireReasonCallback func(key string, reason EvictionReason, value interface{})

// LoaderFunction can be supplied to retrieve an item where a cache miss occurs. Supply an item specific ttl or Duration.Zero
type LoaderFunction func(key string) (data interface{}, ttl time.Duration, err error)

// SimpleCache interface enables a quick-start. Interface for basic usage.
type SimpleCache interface {
	Get(key string) (interface{}, error)
	GetWithTTL(key string) (interface{}, time.Duration, error)
	Set(key string, data interface{}) error
	SetTTL(ttl time.Duration) error
	SetWithTTL(key string, data interface{}, ttl time.Duration) error
	Remove(key string) error
	Close() error
	Purge() error
}

// Cache is a synchronized map of items that can auto-expire once stale
type Cache struct {
	// mutex is shared for all operations that need to be safe
	mutex sync.Mutex
	// ttl is the global ttl for the cache, can be zero (is infinite)
	ttl time.Duration
	// actual item storage
	items map[string]*item
	// lock used to avoid fetching a remote item multiple times
	loaderLock           *singleflight.Group
	expireCallback       ExpireCallback
	expireReasonCallback ExpireReasonCallback
	checkExpireCallback  CheckExpireCallback
	newItemCallback      ExpireCallback
	// the queue is used to have an ordered structure to use for expiration and cleanup.
	priorityQueue          *priorityQueue
	expirationNotification chan bool
	// hasNotified is used to not schedule new expiration processing when an request is already pending.
	hasNotified      bool
	expirationTime   time.Time
	skipTTLExtension bool
	shutdownSignal   chan (chan struct{})
	isShutDown       bool
	loaderFunction   LoaderFunction
	sizeLimit        int
	metrics          Metrics
}

// EvictionReason is an enum that explains why an item was evicted
type EvictionReason int

const (
	// Removed : explicitly removed from cache via API call
	Removed EvictionReason = iota
	// EvictedSize : evicted due to exceeding the cache size
	EvictedSize
	// Expired : the time to live is zero and therefore the item is removed
	Expired
	// Closed : the cache was closed
	Closed
)

const (
	// ErrClosed is raised when operating on a cache where Close() has already been called.
	ErrClosed = constError("cache already closed")
	// ErrNotFound indicates that the requested key is not present in the cache
	ErrNotFound = constError("key not found")
)

type constError string

func (err constError) Error() string {
	return string(err)
}

func (cache *Cache) getItem(key string) (*item, bool, bool) {
	item, exists := cache.items[key]
	if !exists || item.expired() {
		return nil, false, false
	}

	// no need to change priority queue when skipTTLExtension is true or the item will not expire
	if cache.skipTTLExtension || (item.ttl == 0 && cache.ttl == 0) {
		return item, true, false
	}

	if item.ttl == 0 {
		item.ttl = cache.ttl
	}

	item.touch()

	oldExpireTime := cache.priorityQueue.root().expireAt
	cache.priorityQueue.update(item)
	nowExpireTime := cache.priorityQueue.root().expireAt

	expirationNotification := false

	// notify expiration only if the latest expire time is changed
	if (oldExpireTime.IsZero() && !nowExpireTime.IsZero()) || oldExpireTime.After(nowExpireTime) {
		expirationNotification = true
	}
	return item, exists, expirationNotification
}

func (cache *Cache) startExpirationProcessing() {
	timer := time.NewTimer(time.Hour)
	for {
		var sleepTime time.Duration
		cache.mutex.Lock()
		cache.hasNotified = false
		if cache.priorityQueue.Len() > 0 {
			sleepTime = time.Until(cache.priorityQueue.root().expireAt)
			if sleepTime < 0 && cache.priorityQueue.root().expireAt.IsZero() {
				sleepTime = time.Hour
			} else if sleepTime < 0 {
				sleepTime = time.Microsecond
			}
			if cache.ttl > 0 {
				sleepTime = min(sleepTime, cache.ttl)
			}

		} else if cache.ttl > 0 {
			sleepTime = cache.ttl
		} else {
			sleepTime = time.Hour
		}

		cache.expirationTime = time.Now().Add(sleepTime)
		cache.mutex.Unlock()

		timer.Reset(sleepTime)
		select {
		case shutdownFeedback := <-cache.shutdownSignal:
			timer.Stop()
			cache.mutex.Lock()
			if cache.priorityQueue.Len() > 0 {
				cache.evictjob(Closed)
			}
			cache.mutex.Unlock()
			shutdownFeedback <- struct{}{}
			return
		case <-timer.C:
			timer.Stop()
			cache.mutex.Lock()
			if cache.priorityQueue.Len() == 0 {
				cache.mutex.Unlock()
				continue
			}

			cache.cleanjob()
			cache.mutex.Unlock()

		case <-cache.expirationNotification:
			timer.Stop()
			continue
		}
	}
}

func (cache *Cache) checkExpirationCallback(item *item, reason EvictionReason) {
	if cache.expireCallback != nil {
		go cache.expireCallback(item.key, item.data)
	}
	if cache.expireReasonCallback != nil {
		go cache.expireReasonCallback(item.key, reason, item.data)
	}
}

func (cache *Cache) removeItem(item *item, reason EvictionReason) {
	cache.metrics.Evicted++
	cache.checkExpirationCallback(item, reason)
	cache.priorityQueue.remove(item)
	delete(cache.items, item.key)
}

func (cache *Cache) evictjob(reason EvictionReason) {
	// index will only be advanced if the current entry will not be evicted
	i := 0
	for item := cache.priorityQueue.items[i]; ; item = cache.priorityQueue.items[i] {

		cache.removeItem(item, reason)
		if cache.priorityQueue.Len() == 0 {
			return
		}
	}
}

func (cache *Cache) cleanjob() {
	// index will only be advanced if the current entry will not be evicted
	i := 0
	for item := cache.priorityQueue.items[i]; item.expired(); item = cache.priorityQueue.items[i] {

		if cache.checkExpireCallback != nil {
			if !cache.checkExpireCallback(item.key, item.data) {
				item.touch()
				cache.priorityQueue.update(item)
				i++
				if i == cache.priorityQueue.Len() {
					break
				}
				continue
			}
		}

		cache.removeItem(item, Expired)
		if cache.priorityQueue.Len() == 0 {
			return
		}
	}
}

// Close calls Purge after stopping the goroutine that does ttl checking, for a clean shutdown.
// The cache is no longer cleaning up after the first call to Close, repeated calls are safe and return ErrClosed.
func (cache *Cache) Close() error {
	cache.mutex.Lock()
	if !cache.isShutDown {
		cache.isShutDown = true
		cache.mutex.Unlock()
		feedback := make(chan struct{})
		cache.shutdownSignal <- feedback
		<-feedback
		close(cache.shutdownSignal)
		cache.Purge()
	} else {
		cache.mutex.Unlock()
		return ErrClosed
	}
	return nil
}

// Set is a thread-safe way to add new items to the map.
func (cache *Cache) Set(key string, data interface{}) error {
	return cache.SetWithTTL(key, data, ItemExpireWithGlobalTTL)
}

// SetWithTTL is a thread-safe way to add new items to the map with individual ttl.
func (cache *Cache) SetWithTTL(key string, data interface{}, ttl time.Duration) error {
	cache.mutex.Lock()
	if cache.isShutDown {
		cache.mutex.Unlock()
		return ErrClosed
	}
	item, exists, _ := cache.getItem(key)

	oldExpireTime := time.Time{}
	if !cache.priorityQueue.isEmpty() {
		oldExpireTime = cache.priorityQueue.root().expireAt
	}

	if exists {
		item.data = data
		item.ttl = ttl
	} else {
		if cache.sizeLimit != 0 && len(cache.items) >= cache.sizeLimit {
			cache.removeItem(cache.priorityQueue.items[0], EvictedSize)
		}
		item = newItem(key, data, ttl)
		cache.items[key] = item
	}
	cache.metrics.Inserted++

	if item.ttl == 0 {
		item.ttl = cache.ttl
	}

	item.touch()

	if exists {
		cache.priorityQueue.update(item)
	} else {
		cache.priorityQueue.push(item)
	}

	nowExpireTime := cache.priorityQueue.root().expireAt

	cache.mutex.Unlock()
	if !exists && cache.newItemCallback != nil {
		cache.newItemCallback(key, data)
	}

	// notify expiration only if the latest expire time is changed
	if (oldExpireTime.IsZero() && !nowExpireTime.IsZero()) || oldExpireTime.After(nowExpireTime) {
		cache.notifyExpiration()
	}
	return nil
}

// Get is a thread-safe way to lookup items
// Every lookup, also touches the item, hence extending its life
func (cache *Cache) Get(key string) (interface{}, error) {
	return cache.GetByLoader(key, nil)
}

// GetWithTTL has exactly the same behaviour as Get but also returns
// the remaining TTL for a specific item at the moment its retrieved
func (cache *Cache) GetWithTTL(key string) (interface{}, time.Duration, error) {
	return cache.GetByLoaderWithTtl(key, nil)
}

// GetByLoader can take a per key loader function (i.e. to propagate context)
func (cache *Cache) GetByLoader(key string, customLoaderFunction LoaderFunction) (interface{}, error) {
	dataToReturn, _, err := cache.GetByLoaderWithTtl(key, customLoaderFunction)

	return dataToReturn, err
}

// GetByLoaderWithTtl can take a per key loader function (i.e. to propagate context)
func (cache *Cache) GetByLoaderWithTtl(key string, customLoaderFunction LoaderFunction) (interface{}, time.Duration, error) {
	cache.mutex.Lock()
	if cache.isShutDown {
		cache.mutex.Unlock()
		return nil, 0, ErrClosed
	}

	cache.metrics.Hits++
	item, exists, triggerExpirationNotification := cache.getItem(key)

	var dataToReturn interface{}
	ttlToReturn := time.Duration(0)
	if exists {
		cache.metrics.Retrievals++
		dataToReturn = item.data
		if !cache.skipTTLExtension {
			ttlToReturn = item.ttl
		} else {
			ttlToReturn = time.Until(item.expireAt)
		}
		if ttlToReturn < 0 {
			ttlToReturn = 0
		}
	}

	var err error
	if !exists {
		cache.metrics.Misses++
		err = ErrNotFound
	}

	loaderFunction := cache.loaderFunction
	if customLoaderFunction != nil {
		loaderFunction = customLoaderFunction
	}

	if loaderFunction == nil || exists {
		cache.mutex.Unlock()
	}

	if loaderFunction != nil && !exists {
		type loaderResult struct {
			data interface{}
			ttl  time.Duration
		}
		ch := cache.loaderLock.DoChan(key, func() (interface{}, error) {
			// cache is not blocked during io
			invokeData, ttl, err := cache.invokeLoader(key, loaderFunction)
			lr := &loaderResult{
				data: invokeData,
				ttl:  ttl,
			}
			return lr, err
		})
		cache.mutex.Unlock()
		res := <-ch
		dataToReturn = res.Val.(*loaderResult).data
		ttlToReturn = res.Val.(*loaderResult).ttl
		err = res.Err
	}

	if triggerExpirationNotification {
		cache.notifyExpiration()
	}

	return dataToReturn, ttlToReturn, err
}

func (cache *Cache) notifyExpiration() {
	cache.mutex.Lock()
	if cache.hasNotified {
		cache.mutex.Unlock()
		return
	}
	cache.hasNotified = true
	cache.mutex.Unlock()

	cache.expirationNotification <- true
}

func (cache *Cache) invokeLoader(key string, loaderFunction LoaderFunction) (dataToReturn interface{}, ttl time.Duration, err error) {
	dataToReturn, ttl, err = loaderFunction(key)
	if err == nil {
		err = cache.SetWithTTL(key, dataToReturn, ttl)
		if err != nil {
			dataToReturn = nil
		}
	}
	return dataToReturn, ttl, err
}

// Remove removes an item from the cache if it exists, triggers expiration callback when set. Can return ErrNotFound if the entry was not present.
func (cache *Cache) Remove(key string) error {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	if cache.isShutDown {
		return ErrClosed
	}

	object, exists := cache.items[key]
	if !exists {
		return ErrNotFound
	}
	cache.removeItem(object, Removed)

	return nil
}

// Count returns the number of items in the cache. Returns zero when the cache has been closed.
func (cache *Cache) Count() int {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	if cache.isShutDown {
		return 0
	}
	length := len(cache.items)
	return length
}

// GetKeys returns all keys of items in the cache. Returns nil when the cache has been closed.
func (cache *Cache) GetKeys() []string {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	if cache.isShutDown {
		return nil
	}
	keys := make([]string, len(cache.items))
	i := 0
	for k := range cache.items {
		keys[i] = k
		i++
	}
	return keys
}

// GetItems returns a copy of all items in the cache. Returns nil when the cache has been closed.
func (cache *Cache) GetItems() map[string]interface{} {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	if cache.isShutDown {
		return nil
	}
	items := make(map[string]interface{}, len(cache.items))
	for k := range cache.items {
		item, exists, _ := cache.getItem(k)
		if exists {
			items[k] = item.data
		}
	}
	return items
}

// SetTTL sets the global TTL value for items in the cache, which can be overridden at the item level.
func (cache *Cache) SetTTL(ttl time.Duration) error {
	cache.mutex.Lock()

	if cache.isShutDown {
		cache.mutex.Unlock()
		return ErrClosed
	}
	cache.ttl = ttl
	cache.mutex.Unlock()
	cache.notifyExpiration()
	return nil
}

// SetExpirationCallback sets a callback that will be called when an item expires
func (cache *Cache) SetExpirationCallback(callback ExpireCallback) {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	cache.expireCallback = callback
}

// SetExpirationReasonCallback sets a callback that will be called when an item expires, includes reason of expiry
func (cache *Cache) SetExpirationReasonCallback(callback ExpireReasonCallback) {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	cache.expireReasonCallback = callback
}

// SetCheckExpirationCallback sets a callback that will be called when an item is about to expire
// in order to allow external code to decide whether the item expires or remains for another TTL cycle
func (cache *Cache) SetCheckExpirationCallback(callback CheckExpireCallback) {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	cache.checkExpireCallback = callback
}

// SetNewItemCallback sets a callback that will be called when a new item is added to the cache
func (cache *Cache) SetNewItemCallback(callback ExpireCallback) {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	cache.newItemCallback = callback
}

// SkipTTLExtensionOnHit allows the user to change the cache behaviour. When this flag is set to true it will
// no longer extend TTL of items when they are retrieved using Get, or when their expiration condition is evaluated
// using SetCheckExpirationCallback.
func (cache *Cache) SkipTTLExtensionOnHit(value bool) {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	cache.skipTTLExtension = value
}

// SetLoaderFunction allows you to set a function to retrieve cache misses. The signature matches that of the Get function.
// Additional Get calls on the same key block while fetching is in progress (groupcache style).
func (cache *Cache) SetLoaderFunction(loader LoaderFunction) {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	cache.loaderFunction = loader
}

// Purge will remove all entries
func (cache *Cache) Purge() error {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	if cache.isShutDown {
		return ErrClosed
	}
	cache.metrics.Evicted += int64(len(cache.items))
	cache.items = make(map[string]*item)
	cache.priorityQueue = newPriorityQueue()
	return nil
}

// SetCacheSizeLimit sets a limit to the amount of cached items.
// If a new item is getting cached, the closes item to being timed out will be replaced
// Set to 0 to turn off
func (cache *Cache) SetCacheSizeLimit(limit int) {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	cache.sizeLimit = limit
}

// NewCache is a helper to create instance of the Cache struct
func NewCache() *Cache {

	shutdownChan := make(chan chan struct{})

	cache := &Cache{
		items:                  make(map[string]*item),
		loaderLock:             &singleflight.Group{},
		priorityQueue:          newPriorityQueue(),
		expirationNotification: make(chan bool, 1),
		expirationTime:         time.Now(),
		shutdownSignal:         shutdownChan,
		isShutDown:             false,
		loaderFunction:         nil,
		sizeLimit:              0,
		metrics:                Metrics{},
	}
	go cache.startExpirationProcessing()
	return cache
}

// GetMetrics exposes the metrics of the cache. This is a snapshot copy of the metrics.
func (cache *Cache) GetMetrics() Metrics {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	return cache.metrics
}

// Touch resets the TTL of the key when it exists, returns ErrNotFound if the key is not present.
func (cache *Cache) Touch(key string) error {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	item, exists := cache.items[key]
	if !exists {
		return ErrNotFound
	}
	item.touch()
	return nil
}

func min(duration time.Duration, second time.Duration) time.Duration {
	if duration < second {
		return duration
	}
	return second
}
