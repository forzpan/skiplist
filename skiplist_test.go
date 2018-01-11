package skiplist

import (
	"math/rand"
	"sync"
	"testing"
	"time"
)

var wg sync.WaitGroup

type item struct {
	key int64
	val []byte
}

func (it item) Less(than Item) bool {
	return it.key < than.(item).key
}

func insert(list *SkipList) {
	defer wg.Done()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < 10000; i++ {
		it := item{key: r.Int63n(10000)}
		list.Set(it)
	}
}

func checkError(list *SkipList) bool {
	for i := int8(0); i < list.maxLevel; i++ {
		if list.head.tower[i] == nil {
			continue
		}
		cur := (*node)(list.head.tower[i])
		for cur.tower[i] != nil {
			next := (*node)(cur.tower[i])
			if !cur.item.Less(next.item) {
				return true
			}
			cur = next
		}
	}
	return false
}

func TestSkiplist(t *testing.T) {
	var list *SkipList
	list, _ = New(8)

	wg.Add(4)
	go insert(list)
	go insert(list)
	go insert(list)
	go insert(list)
	wg.Wait()

	if checkError(list) {
		t.Fatal(`wrong list`)
	}
}
