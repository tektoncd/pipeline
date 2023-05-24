# TTLCache - an in-memory cache with expiration

**Although v2 of ttlcache is not yet deprecated, v3 should be used as it 
contains quite a few additions and improvements.**

TTLCache is a simple key/value cache in golang with the following functions:

1. Expiration of items based on time, or custom function
2. Loader function to retrieve missing keys can be provided. Additional `Get` calls on the same key block while fetching is in progress (groupcache style).
3. Individual expiring time or global expiring time, you can choose
4. Auto-Extending expiration on `Get` -or- DNS style TTL, see `SkipTTLExtensionOnHit(bool)`
5. Can trigger callback on key expiration
6. Cleanup resources by calling `Close()` at end of lifecycle.
7. Thread-safe with comprehensive testing suite. This code is in production at bol.com on critical systems.

Note (issue #25): by default, due to historic reasons, the TTL will be reset on each cache hit and you need to explicitly configure the cache to use a TTL that will not get extended.

## Usage

`go get github.com/jellydator/ttlcache/v2`

You can copy it as a full standalone demo program. The first snippet is basic usage, where the second exploits more options in the cache.

Basic:
```go
package main

import (
	"fmt"
	"time"

	"github.com/jellydator/ttlcache/v2"
)

var notFound = ttlcache.ErrNotFound

func main() {
	var cache ttlcache.SimpleCache = ttlcache.NewCache()

	cache.SetTTL(time.Duration(10 * time.Second))
	cache.Set("MyKey", "MyValue")
	cache.Set("MyNumber", 1000)

	if val, err := cache.Get("MyKey"); err != notFound {
		fmt.Printf("Got it: %s\n", val)
	}

	cache.Remove("MyNumber")
	cache.Purge()
	cache.Close()
}
```

Advanced:
```go
package main

import (
	"fmt"
	"time"

	"github.com/jellydator/ttlcache/v2"
)

var (
	notFound = ttlcache.ErrNotFound
	isClosed = ttlcache.ErrClosed
)

func main() {
	newItemCallback := func(key string, value interface{}) {
		fmt.Printf("New key(%s) added\n", key)
	}
	checkExpirationCallback := func(key string, value interface{}) bool {
		if key == "key1" {
			// if the key equals "key1", the value
			// will not be allowed to expire
			return false
		}
		// all other values are allowed to expire
		return true
	}

	expirationCallback := func(key string, reason ttlcache.EvictionReason, value interface{}) {
		fmt.Printf("This key(%s) has expired because of %s\n", key, reason)
	}

	loaderFunction := func(key string) (data interface{}, ttl time.Duration, err error) {
		ttl = time.Second * 300
		data, err = getFromNetwork(key)

		return data, ttl, err
	}

	cache := ttlcache.NewCache()
	cache.SetTTL(time.Duration(10 * time.Second))
	cache.SetExpirationReasonCallback(expirationCallback)
	cache.SetLoaderFunction(loaderFunction)
	cache.SetNewItemCallback(newItemCallback)
	cache.SetCheckExpirationCallback(checkExpirationCallback)
	cache.SetCacheSizeLimit(2)

	cache.Set("key", "value")
	cache.SetWithTTL("keyWithTTL", "value", 10*time.Second)

	if value, exists := cache.Get("key"); exists == nil {
		fmt.Printf("Got value: %v\n", value)
	}
	count := cache.Count()
	if result := cache.Remove("keyNNN"); result == notFound {
		fmt.Printf("Not found, %d items left\n", count)
	}

	cache.Set("key6", "value")
	cache.Set("key7", "value")
	metrics := cache.GetMetrics()
	fmt.Printf("Total inserted: %d\n", metrics.Inserted)

	cache.Close()

}

func getFromNetwork(key string) (string, error) {
	time.Sleep(time.Millisecond * 30)
	return "value", nil
}
```

### TTLCache - Some design considerations

1. The complexity of the current cache is already quite high. Therefore not all requests can be implemented in a straight-forward manner.
2. The locking should be done only in the exported functions and `startExpirationProcessing` of the Cache struct. Else data races can occur or recursive locks are needed, which are both unwanted.
3. I prefer correct functionality over fast tests. It's ok for new tests to take seconds to proof something.

### Original Project

TTLCache was forked from [wunderlist/ttlcache](https://github.com/wunderlist/ttlcache) to add extra functions not avaiable in the original scope.
The main differences are:

1. A item can store any kind of object, previously, only strings could be saved
2. Optionally, you can add callbacks too: check if a value should expire, be notified if a value expires, and be notified when new values are added to the cache
3. The expiration can be either global or per item
4. Items can exist without expiration time (time.Zero)
5. Expirations and callbacks are realtime. Don't have a pooling time to check anymore, now it's done with a heap.
6. A cache count limiter
