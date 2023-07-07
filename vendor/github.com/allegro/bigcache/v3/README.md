# BigCache [![Build Status](https://github.com/allegro/bigcache/workflows/build/badge.svg)](https://github.com/allegro/bigcache/actions?query=workflow%3Abuild)&nbsp;[![Coverage Status](https://coveralls.io/repos/github/allegro/bigcache/badge.svg?branch=master)](https://coveralls.io/github/allegro/bigcache?branch=master)&nbsp;[![GoDoc](https://godoc.org/github.com/allegro/bigcache/v3?status.svg)](https://godoc.org/github.com/allegro/bigcache/v3)&nbsp;[![Go Report Card](https://goreportcard.com/badge/github.com/allegro/bigcache/v3)](https://goreportcard.com/report/github.com/allegro/bigcache/v3)

Fast, concurrent, evicting in-memory cache written to keep big number of entries without impact on performance.
BigCache keeps entries on heap but omits GC for them. To achieve that, operations on byte slices take place,
therefore entries (de)serialization in front of the cache will be needed in most use cases.

Requires Go 1.12 or newer.

## Usage

### Simple initialization

```go
import (
	"fmt"
	"context"
	"github.com/allegro/bigcache/v3"
)

cache, _ := bigcache.New(context.Background(), bigcache.DefaultConfig(10 * time.Minute))

cache.Set("my-unique-key", []byte("value"))

entry, _ := cache.Get("my-unique-key")
fmt.Println(string(entry))
```

### Custom initialization

When cache load can be predicted in advance then it is better to use custom initialization because additional memory
allocation can be avoided in that way.

```go
import (
	"log"

	"github.com/allegro/bigcache/v3"
)

config := bigcache.Config {
		// number of shards (must be a power of 2)
		Shards: 1024,

		// time after which entry can be evicted
		LifeWindow: 10 * time.Minute,

		// Interval between removing expired entries (clean up).
		// If set to <= 0 then no action is performed.
		// Setting to < 1 second is counterproductive — bigcache has a one second resolution.
		CleanWindow: 5 * time.Minute,

		// rps * lifeWindow, used only in initial memory allocation
		MaxEntriesInWindow: 1000 * 10 * 60,

		// max entry size in bytes, used only in initial memory allocation
		MaxEntrySize: 500,

		// prints information about additional memory allocation
		Verbose: true,

		// cache will not allocate more memory than this limit, value in MB
		// if value is reached then the oldest entries can be overridden for the new ones
		// 0 value means no size limit
		HardMaxCacheSize: 8192,

		// callback fired when the oldest entry is removed because of its expiration time or no space left
		// for the new entry, or because delete was called. A bitmask representing the reason will be returned.
		// Default value is nil which means no callback and it prevents from unwrapping the oldest entry.
		OnRemove: nil,

		// OnRemoveWithReason is a callback fired when the oldest entry is removed because of its expiration time or no space left
		// for the new entry, or because delete was called. A constant representing the reason will be passed through.
		// Default value is nil which means no callback and it prevents from unwrapping the oldest entry.
		// Ignored if OnRemove is specified.
		OnRemoveWithReason: nil,
	}

cache, initErr := bigcache.New(context.Background(), config)
if initErr != nil {
	log.Fatal(initErr)
}

cache.Set("my-unique-key", []byte("value"))

if entry, err := cache.Get("my-unique-key"); err == nil {
	fmt.Println(string(entry))
}
```

### `LifeWindow` & `CleanWindow`

1. `LifeWindow` is a time. After that time, an entry can be called dead but not deleted.

2. `CleanWindow` is a time. After that time, all the dead entries will be deleted, but not the entries that still have life.

## [Benchmarks](https://github.com/allegro/bigcache-bench)

Three caches were compared: bigcache, [freecache](https://github.com/coocood/freecache) and map.
Benchmark tests were made using an
i7-6700K CPU @ 4.00GHz with 32GB of RAM on Ubuntu 18.04 LTS (5.2.12-050212-generic).

Benchmarks source code can be found [here](https://github.com/allegro/bigcache-bench)

### Writes and reads

```bash
go version
go version go1.13 linux/amd64

go test -bench=. -benchmem -benchtime=4s ./... -timeout 30m
goos: linux
goarch: amd64
pkg: github.com/allegro/bigcache/v3/caches_bench
BenchmarkMapSet-8                     	12999889	       376 ns/op	     199 B/op	       3 allocs/op
BenchmarkConcurrentMapSet-8           	 4355726	      1275 ns/op	     337 B/op	       8 allocs/op
BenchmarkFreeCacheSet-8               	11068976	       703 ns/op	     328 B/op	       2 allocs/op
BenchmarkBigCacheSet-8                	10183717	       478 ns/op	     304 B/op	       2 allocs/op
BenchmarkMapGet-8                     	16536015	       324 ns/op	      23 B/op	       1 allocs/op
BenchmarkConcurrentMapGet-8           	13165708	       401 ns/op	      24 B/op	       2 allocs/op
BenchmarkFreeCacheGet-8               	10137682	       690 ns/op	     136 B/op	       2 allocs/op
BenchmarkBigCacheGet-8                	11423854	       450 ns/op	     152 B/op	       4 allocs/op
BenchmarkBigCacheSetParallel-8        	34233472	       148 ns/op	     317 B/op	       3 allocs/op
BenchmarkFreeCacheSetParallel-8       	34222654	       268 ns/op	     350 B/op	       3 allocs/op
BenchmarkConcurrentMapSetParallel-8   	19635688	       240 ns/op	     200 B/op	       6 allocs/op
BenchmarkBigCacheGetParallel-8        	60547064	        86.1 ns/op	     152 B/op	       4 allocs/op
BenchmarkFreeCacheGetParallel-8       	50701280	       147 ns/op	     136 B/op	       3 allocs/op
BenchmarkConcurrentMapGetParallel-8   	27353288	       175 ns/op	      24 B/op	       2 allocs/op
PASS
ok  	github.com/allegro/bigcache/v3/caches_bench	256.257s
```

Writes and reads in bigcache are faster than in freecache.
Writes to map are the slowest.

### GC pause time

```bash
go version
go version go1.13 linux/amd64

go run caches_gc_overhead_comparison.go

Number of entries:  20000000
GC pause for bigcache:  1.506077ms
GC pause for freecache:  5.594416ms
GC pause for map:  9.347015ms
```

```
go version
go version go1.13 linux/arm64

go run caches_gc_overhead_comparison.go
Number of entries:  20000000
GC pause for bigcache:  22.382827ms
GC pause for freecache:  41.264651ms
GC pause for map:  72.236853ms
```

Test shows how long are the GC pauses for caches filled with 20mln of entries.
Bigcache and freecache have very similar GC pause time.

### Memory usage

You may encounter system memory reporting what appears to be an exponential increase, however this is expected behaviour. Go runtime allocates memory in chunks or 'spans' and will inform the OS when they are no longer required by changing their state to 'idle'. The 'spans' will remain part of the process resource usage until the OS needs to repurpose the address. Further reading available [here](https://utcc.utoronto.ca/~cks/space/blog/programming/GoNoMemoryFreeing).

## How it works

BigCache relies on optimization presented in 1.5 version of Go ([issue-9477](https://github.com/golang/go/issues/9477)).
This optimization states that if map without pointers in keys and values is used then GC will omit its content.
Therefore BigCache uses `map[uint64]uint32` where keys are hashed and values are offsets of entries.

Entries are kept in byte slices, to omit GC again.
Byte slices size can grow to gigabytes without impact on performance
because GC will only see single pointer to it.

### Collisions

BigCache does not handle collisions. When new item is inserted and it's hash collides with previously stored item, new item overwrites previously stored value.

## Bigcache vs Freecache

Both caches provide the same core features but they reduce GC overhead in different ways.
Bigcache relies on `map[uint64]uint32`, freecache implements its own mapping built on
slices to reduce number of pointers.

Results from benchmark tests are presented above.
One of the advantage of bigcache over freecache is that you don’t need to know
the size of the cache in advance, because when bigcache is full,
it can allocate additional memory for new entries instead of
overwriting existing ones as freecache does currently.
However hard max size in bigcache also can be set, check [HardMaxCacheSize](https://godoc.org/github.com/allegro/bigcache#Config).

## HTTP Server

This package also includes an easily deployable HTTP implementation of BigCache, which can be found in the [server](/server) package.

## More

Bigcache genesis is described in allegro.tech blog post: [writing a very fast cache service in Go](http://allegro.tech/2016/03/writing-fast-cache-service-in-go.html)

## License

BigCache is released under the Apache 2.0 license (see [LICENSE](LICENSE))
