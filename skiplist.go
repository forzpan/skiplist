//不支持delete
//不支持同Item多次set
//支持并发set

package skiplist

import (
	"errors"
	"math"
	"math/rand"
	"sync/atomic"
	"time"
	"unsafe"
)

var (
	randSource  rand.Source //自定义随机数,不使用globalRand,globalRand有锁
	levelLimit  int8        //允许最大层高(0, 64]
	probability float64     //概率因子[0, 1]
	probTable   []float64   //层高概率表
)

func init() {
	randSource = rand.New(rand.NewSource(time.Now().UnixNano()))
	levelLimit = 21
	probability = 1 / math.E
	probTable = make([]float64, levelLimit)
	for i := int8(0); i < levelLimit; i++ {
		probTable[i] = math.Pow(probability, float64(i))
	}
}

// Item represents a single object in the skipList.
type Item interface {
	// Less tests whether the current item is less than the given argument.
	// This must provide a strict weak ordering.
	// If !a.Less(b) && !b.Less(a), we treat this to mean a == b (i.e. we can only hold one of either a or b in the skipList).
	Less(than Item) bool
}

type node struct {
	item  Item
	tower []unsafe.Pointer
}

type SkipList struct {
	head     *node //无值头结点
	maxLevel int8  //最大层高
}

func New(maxLevel int8) (*SkipList, error) {
	if maxLevel > levelLimit {
		return nil, errors.New("maxLevel is too big")
	}
	return &SkipList{
		head:     &node{tower: make([]unsafe.Pointer, maxLevel)},
		maxLevel: maxLevel,
	}, nil
}

func (list *SkipList) Get(item Item) Item {

	var cur *node = list.head
	var next *node

	for i := list.maxLevel - 1; i >= 0; i-- {
		next = (*node)(cur.tower[i])
		for next != nil && next.item.Less(item) {
			cur, next = next, (*node)(next.tower[i])
		}
	}

	//到这里保证 next == nil || !next.item.Less(item)
	if next != nil && !item.Less(next.item) {
		return next.item
	}

	return nil
}

func (list *SkipList) randLevel() (level int8) {
	r := float64(randSource.Int63()) / (1 << 63)

	level = 1
	for level < list.maxLevel && r < probTable[level] {
		level++
	}
	return
}

// 确定item位置 返回查询路线和查询时刻上限
func (list *SkipList) getSearchPath(item Item) ([]*node, *node) {
	//前节点集合,每层一个。根据新节点的随机层数使用
	var prevs []*node = make([]*node, list.maxLevel)

	var cur *node = list.head
	var next *node

	for i := list.maxLevel - 1; i >= 0; i-- {
		next = (*node)(cur.tower[i])
		for next != nil && next.item.Less(item) {
			cur, next = next, (*node)(next.tower[i])
		}
		prevs[i] = cur
	}

	return prevs, next
}

func (list *SkipList) Set(item Item) error {

	prevs, next := list.getSearchPath(item)

	//找到
	if next != nil && !item.Less(next.item) {
		return errors.New("item is exists")
	}

	//没找到，创建新节点
	newnode := &node{
		item:  item,
		tower: make([]unsafe.Pointer, list.randLevel()),
	}

	for i := 0; i < len(newnode.tower); i++ {
	F:
		nodeptr := (prevs[i].tower[i])
		next := (*node)(nodeptr)
		if next == nil || item.Less(next.item) {
			newnode.tower[i] = nodeptr
			//第i层前一个节点prevs[i]的tower[i]的内存地址内的指针值还是原下一个节点地址，没有变化
			if ok := atomic.CompareAndSwapPointer(&(prevs[i].tower[i]), nodeptr, unsafe.Pointer(newnode)); ok {
				continue //第i层成功，继续上一层
			}
			//第i层前一个节点prevs[i]的当前版本的tower[i]的内存地址内的指针值已经不是原节点，变化了，尝试重新获取前驱继续insert操作
			next = (*node)(prevs[i].tower[i])
			//新插入item小于准备插入的item，修改前驱节点
			if next.item.Less(item) {
				prevs[i] = next
			}
			goto F
		}
		//下个item小于准备插入的item，修改前驱节点
		if next.item.Less(item) {
			prevs[i] = next
			goto F
		}
		//可能其他进程set了这个item
		return errors.New("item is setted by other gothread")
	}

	return nil
}
