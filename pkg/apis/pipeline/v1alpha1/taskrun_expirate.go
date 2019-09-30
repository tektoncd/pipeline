package v1alpha1

import (
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

const (
	NoExpiration      time.Duration = -1 //没有过期标识
	defaultExpiration time.Duration = 0  //默认不过期（过期时间以ns为单位）
)

type Item struct {
	Object     interface{} //数据项
	Expiration int64       //数据项过期时间(0永不过期)
}

type Cache struct {
	defaultExpiration time.Duration //如果数据项没有指定过期时使用
	items             map[string]Item
	mu                sync.RWMutex  //读写锁
	gcInterval        time.Duration //gc周期
	stopGc            chan bool     //停止gc管道标识
}

func (item Item) IsExpired() bool {
	if item.Expiration == 0 {
		return false
	}
	return time.Now().UnixNano() > item.Expiration //如果当前时间超则过期
}

//循环gc
func (c *Cache) gcLoop() {
	ticker := time.NewTicker(c.gcInterval) //初始化一个定时器
	for {
		select {
		case <-ticker.C:
			c.DeleteExpired()
		case <-c.stopGc:
			ticker.Stop()
			return
		}
	}
}

//过期缓存删除
func (c *Cache) DeleteExpired() {
	now := time.Now().UnixNano()
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, v := range c.items {
		if v.Expiration > 0 && now > v.Expiration {
			c.delete(k)
		}
	}
}

//删除
func (c *Cache) delete(k string) {
	delete(c.items, k)
}

//删除操作
func (c *Cache) Delete(k string) {
	c.mu.Lock()
	c.delete(k)
	defer c.mu.Unlock()
}

//设置缓存数据项，存在则直接覆盖
func (c *Cache) Set(k string, v interface{}, d time.Duration) {
	var e int64
	if d == defaultExpiration {
		d = c.defaultExpiration
	}
	if d > 0 {
		e = time.Now().Add(d).UnixNano()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[k] = Item{
		Object:     v,
		Expiration: e,
	}
}

//设置数据项，无锁操作
func (c *Cache) set(k string, v interface{}, d time.Duration) {
	var e int64
	if d == defaultExpiration {
		d = c.defaultExpiration
	}
	if d > 0 {
		e = time.Now().Add(d).UnixNano()
	}
	c.items[k] = Item{
		Object:     v,
		Expiration: e,
	}
}

//获取数据项，并判断数据项是否过期
func (c *Cache) get(k string) (interface{}, bool) {
	item, found := c.items[k]
	if !found || item.IsExpired() {
		return nil, false
	}
	return item.Object, true
}

//新增操作,如果数据项已存在，则报错
func (c *Cache) Add(k string, v interface{}, d time.Duration) error {
	c.mu.Lock()
	_, found := c.get(k)
	if found {
		c.mu.Unlock()
		return fmt.Errorf("Item %s already exists", k)
	}
	c.set(k, v, d)
	c.mu.Unlock()
	return nil
}

//获取缓存操作
func (c *Cache) Get(k string) (interface{}, bool) {
	c.mu.RLock()
	item, found := c.items[k]
	if !found || item.IsExpired() {
		c.mu.RUnlock()
		return nil, false
	}
	c.mu.RUnlock()
	return item.Object, true
}

//替换操作
func (c *Cache) Replace(k string, v interface{}, d time.Duration) error {
	c.mu.Lock()
	_, found := c.get(k)
	if !found {
		c.mu.Unlock()
		return fmt.Errorf("Item %s does't exists", k)
	}
	c.set(k, v, d)
	c.mu.Unlock()
	return nil
}

//将缓存数据写入io.Writer中
func (c *Cache) Save(w io.Writer) (err error) {
	enc := gob.NewEncoder(w)
	defer func() {
		if x := recover(); x != nil {
			err = fmt.Errorf("Error Registering item types with gob library")
		}
	}()
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, v := range c.items {
		gob.Register(v.Object)
	}
	err = enc.Encode(&c.items)
	return
}

//序列化到文件
func (c *Cache) SaveToFile(filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	if err = c.Save(f); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

//从io.Reader读取
func (c *Cache) Load(r io.Reader) error {
	dec := gob.NewDecoder(r)
	items := make(map[string]Item, 0)
	err := dec.Decode(&items)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, v := range items {
		obj, ok := c.items[k]
		if !ok || obj.IsExpired() {
			c.items[k] = v
		}
	}
	return nil
}

//从文件中读取
func (c *Cache) LoadFromFile(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	if err = c.Load(f); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

//返回缓存中数据项的数量
func (c *Cache) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

//清空缓存
func (c *Cache) Flush() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = map[string]Item{}
}

//停止gc
func (c *Cache) StopGc() {
	c.stopGc <- true
}

//新建一个缓存系统
func NewCache(defaultExpiration, gcInterval time.Duration) (c *Cache) {
	c = &Cache{
		defaultExpiration: defaultExpiration,
		gcInterval:        gcInterval,
		items:             map[string]Item{},
		stopGc:            make(chan bool),
	}
	go c.gcLoop()
	return
}
