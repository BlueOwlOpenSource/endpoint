package endpoint

import (
	"reflect"
	"sync"
)

type typeCode int
type typeTuple struct {
	t    reflect.Type
	code typeCode
}

var typeCounter = 0
var lock sync.Mutex
var typeMap = make(map[string][]typeTuple)
var reverseMap = make(map[typeCode]reflect.Type)

type noType bool

const noTypeExampleValue noType = false

var noTypeCode = GetTypeCode(noTypeExampleValue)

// GetTypeCode maps Go types to integers.  It is exported for
// testing purposes.
func GetTypeCode(a interface{}) typeCode {
	if a == nil {
		panic("nil has no type")
	}
	t, isType := a.(reflect.Type)
	if !isType {
		t = reflect.TypeOf(a)
	}
	lock.Lock()
	defer lock.Unlock()
	tlist, found := typeMap[t.String()]
	if !found {
		tlist = make([]typeTuple, 0)
	}
	for _, tt := range tlist {
		if tt.t == t {
			return tt.code
		}
	}
	typeCounter++
	tc := typeCode(typeCounter)
	typeMap[t.String()] = append(tlist, typeTuple{
		t:    t,
		code: tc,
	})
	reverseMap[tc] = t
	return tc
}

// Type returns the reflect.Type for this typeCode
func (tc typeCode) Type() reflect.Type {
	lock.Lock()
	defer lock.Unlock()
	return reverseMap[tc]
}
