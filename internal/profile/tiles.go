package profile

import (
	"container/list"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/nils-witt/wireless-terrain-planner/server/internal/geotiff"
)

// tileKey identifies a 1 km x 1 km DGM1 tile by the kilometre coordinates of
// its south-west corner in EPSG:25832.
type tileKey struct{ x, y int }

func (k tileKey) filename() string {
	return fmt.Sprintf("dgm1_32_%d_%d_1_nw_2021.tif", k.x, k.y)
}

// tileCache is a bounded LRU of decoded tiles. Concurrent requests for the
// same tile share a single load, and failed loads are not cached, so a tile
// that appears on disk later is picked up.
type tileCache struct {
	dir      string
	capacity int
	load     func(path string) (*geotiff.Raster, error)

	mu      sync.Mutex
	entries map[tileKey]*list.Element
	lru     *list.List // front = most recently used
}

type cacheEntry struct {
	key    tileKey
	once   sync.Once
	raster *geotiff.Raster
	err    error
}

func newTileCache(dir string, capacity int) *tileCache {
	return &tileCache{
		dir:      dir,
		capacity: max(capacity, 1),
		load:     geotiff.Open,
		entries:  make(map[tileKey]*list.Element),
		lru:      list.New(),
	}
}

func (c *tileCache) get(k tileKey) (*geotiff.Raster, error) {
	c.mu.Lock()
	el, ok := c.entries[k]
	if ok {
		c.lru.MoveToFront(el)
	} else {
		el = c.lru.PushFront(&cacheEntry{key: k})
		c.entries[k] = el
		for c.lru.Len() > c.capacity {
			oldest := c.lru.Back()
			c.lru.Remove(oldest)
			delete(c.entries, oldest.Value.(*cacheEntry).key)
		}
	}
	e := el.Value.(*cacheEntry)
	c.mu.Unlock()

	e.once.Do(func() {
		e.raster, e.err = c.load(filepath.Join(c.dir, k.filename()))
	})
	if e.err != nil {
		c.mu.Lock()
		if cur, ok := c.entries[k]; ok && cur.Value.(*cacheEntry) == e {
			c.lru.Remove(cur)
			delete(c.entries, k)
		}
		c.mu.Unlock()
		return nil, e.err
	}
	return e.raster, nil
}
