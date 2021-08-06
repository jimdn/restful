package restful

import (
	"container/list"
	"fmt"
	"github.com/globalsign/mgo"
	"reflect"
	"strings"
	"sync"
	"time"
)

// Index describes the definition of an index
type Index struct {
	Key    []string // Index key fields; prefix name with dash (-) for descending order
	Unique bool     // Prevent two documents from having the same index key
}

func getIndexMapKey(db, table string) string {
	return fmt.Sprintf("%s|%s", db, table)
}

// list to ensure index
var indexEnsureList *IndexEnsureList

func getIndexEnsureList() *IndexEnsureList {
	if indexEnsureList == nil {
		indexEnsureList = new(IndexEnsureList).Init()
	}
	return indexEnsureList
}

// IndexEnsureList is a structure describes a list of indices to ensure
type IndexEnsureList struct {
	sync.Mutex
	// using map to check elem already in list or not
	// key: db|table
	indexToEnsureMap map[string]IndexToEnsureStruct
	// using list to store elem, FIFO
	indexToEnsureList *list.List
}

// IndexToEnsureStruct defines where and how to create the index
type IndexToEnsureStruct struct {
	DB        string
	Table     string
	Processor *Processor
}

// Init init the IndexEnsureList
func (l *IndexEnsureList) Init() *IndexEnsureList {
	l.indexToEnsureMap = make(map[string]IndexToEnsureStruct)
	l.indexToEnsureList = list.New()
	return l
}

// Push push an index into IndexEnsureList
func (l *IndexEnsureList) Push(idx *IndexToEnsureStruct) {
	if idx == nil {
		return
	}
	l.Lock()
	defer l.Unlock()
	k := getIndexMapKey(idx.DB, idx.Table)
	if _, ok := l.indexToEnsureMap[k]; ok {
		// if exists, return directly
		return
	}
	l.indexToEnsureMap[k] = *idx
	l.indexToEnsureList.PushBack(k)
}

// Pop pop an index out of IndexEnsureList
func (l *IndexEnsureList) Pop() *IndexToEnsureStruct {
	l.Lock()
	defer l.Unlock()
	e := l.indexToEnsureList.Front()
	if e == nil {
		return nil
	}
	k := e.Value.(string)
	idx, ok := l.indexToEnsureMap[k]
	if !ok {
		l.indexToEnsureList.Remove(e)
		return nil
	}
	delete(l.indexToEnsureMap, k)
	l.indexToEnsureList.Remove(e)
	return &idx
}

// Cache to store index that has been ensured
// 600 seconds expired, ensure again
var indexEnsuredMap *IndexEnsuredMap

func getIndexEnsuredMap() *IndexEnsuredMap {
	if indexEnsuredMap == nil {
		indexEnsuredMap = &IndexEnsuredMap{
			M: make(map[string]int64),
		}
	}
	return indexEnsuredMap
}

// IndexEnsuredMap cache to store index that has been ensured
type IndexEnsuredMap struct {
	sync.RWMutex
	// key: db|table
	M map[string]int64
}

// Set add an index into the cache
func (s *IndexEnsuredMap) Set(k string) {
	s.Lock()
	defer s.Unlock()
	s.M[k] = time.Now().Unix() + 600
}

// Exist check whether an index exists or not
func (s *IndexEnsuredMap) Exist(k string) bool {
	now := time.Now().Unix()
	s.RLock()
	defer s.RUnlock()
	if v, ok := s.M[k]; ok {
		if v > now {
			return true
		}
	}
	return false
}

func ensureIndexTask() {
	dbs := gCfg.MgoSess.Clone()
	defer dbs.Close()
	for {
		time.Sleep(1 * time.Second)

		// get elem from list
		idx := getIndexEnsureList().Pop()
		if idx == nil || idx.DB == "" || idx.Table == "" || idx.Processor == nil || len(idx.Processor.Indexes) == 0 {
			continue
		}
		// ensure index
		k := getIndexMapKey(idx.DB, idx.Table)
		if getIndexEnsuredMap().Exist(k) {
			continue
		}

		dbc := dbs.DB(idx.DB).C(idx.Table)
		indexesInDB, err := dbc.Indexes()
		if err != nil {
			if strings.Contains(err.Error(), "ns does not exist") {
				continue
			}
			Log.Warnf("db=%s table=%s GetIndexes err: %v", idx.DB, idx.Table, err)
			continue
		}
		for i := 0; i < len(idx.Processor.Indexes); i++ {
			existInDB := false
			for j := 0; j < len(indexesInDB); j++ {
				if reflect.DeepEqual(idx.Processor.Indexes[i].Key, indexesInDB[j].Key) && idx.Processor.Indexes[i].Unique == indexesInDB[j].Unique {
					existInDB = true
					break
				}
			}
			if !existInDB {
				err := dbc.EnsureIndex(mgo.Index{
					Key:        idx.Processor.Indexes[i].Key,
					Unique:     idx.Processor.Indexes[i].Unique,
					Background: true,
				})
				if err != nil {
					Log.Warnf("db=%s table=%s EnsureIndex(%v) err: %v", idx.DB, idx.Table, idx.Processor.Indexes[i].Key, err)
				}
			}
		}
		getIndexEnsuredMap().Set(k)
	}
}
