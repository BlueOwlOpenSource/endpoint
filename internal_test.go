package endpoint

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const lastT = true
const lastF = false
const staticT = true
const staticF = false
const panicT = true
const panicF = false
const nowPastT = true
const nowPastF = false

type intType1 int
type intType2 int
type intType3 int
type intType4 int
type intType5 int
type intType6 int
type intType7 int

type stringA string
type stringB string
type stringC string
type stringD string
type stringE string
type stringF string
type stringG string
type stringH string
type stringI string
type stringJ string

var tsiwfsFunc1a = func() intType3 { return 9 }
var tsiwfsFunc2a = func(w http.ResponseWriter, r *http.Request) {}

type interfaceI interface {
	I() int
}
type interfaceJ interface {
	I() int
}
type interfaceK interface {
	I() int
}
type doesI struct {
	i int
}

func (di *doesI) I() int { return di.i * 2 }

type doesJ struct {
	j int
}

func (dj *doesJ) I() int { return dj.j * 3 }

var characterizeTests = []struct {
	name                   string
	fn                     interface{}
	expectedTypeCode       typeCode
	last                   bool
	pastStatic             bool
	expectedToPanic        bool
	expectedNowPastStatic  bool
	expectedInputParams    []typeCode // recevied from above
	expectedOutputParams   []typeCode // going down
	expectedReturnParams   []typeCode // going up
	expectedReturnedParams []typeCode // received from below
}{
	{
		"tsiwfsFunc1",
		tsiwfsFunc1a,
		staticInjectorFunc,
		lastF, staticF, panicF, nowPastF,
		nil, []typeCode{intType3TC},
		nil, nil,
	},
	{
		"tsiwfsFunc2",
		tsiwfsFunc2a,
		endpointFunc,
		lastT, staticF, panicF, nowPastT,
		[]typeCode{responseWriterTC, requestTC}, nil,
		nil, nil,
	},
	{
		"static injector",
		func(int) string { return "" },
		staticInjectorFunc,
		lastF, staticF, panicF, nowPastF,
		[]typeCode{intTC}, []typeCode{stringTC},
		nil, nil,
	},
	{
		"minimal injector",
		func() {},
		injectorFunc,
		lastF, staticT, panicF, nowPastT,
		[]typeCode{}, []typeCode{},
		nil, nil,
	},
	{
		"minimal injector annotated as not static",
		AnnotateNotStatic(func() {}),
		injectorFunc,
		lastF, staticF, panicF, nowPastT,
		[]typeCode{}, []typeCode{},
		nil, nil,
	},
	{
		"static injector",
		func(int) string { return "" },
		injectorFunc,
		lastF, staticT, panicF, nowPastT,
		[]typeCode{intTC}, []typeCode{stringTC},
		nil, nil,
	},
	{
		"minimal fallible injector",
		func() TerminalError { return nil },
		fallibleInjectorFunc,
		lastF, staticF, panicF, nowPastT,
		[]typeCode{}, nil,
		[]typeCode{errorTC}, nil,
	},
	{
		"fallible injector",
		func(int, string) (TerminalError, string) { return nil, "" },
		fallibleInjectorFunc,
		lastF, staticF, panicF, nowPastT,
		[]typeCode{intTC, stringTC}, []typeCode{stringTC},
		[]typeCode{errorTC}, nil,
	},
	{
		"endpoint",
		func(int, string) string { return "" },
		endpointFunc,
		lastT, staticT, panicF, nowPastT,
		[]typeCode{intTC, stringTC}, nil,
		[]typeCode{stringTC}, nil,
	},
	{
		"consumer of value",
		func(int, string) {},
		injectorFunc,
		lastF, staticF, panicF, nowPastT,
		[]typeCode{intTC, stringTC}, nil,
		nil, nil,
	},
	{
		"injector switch over from static",
		func(*http.Request) {},
		injectorFunc,
		lastF, staticF, panicF, nowPastT,
		[]typeCode{requestTC}, nil,
		nil, nil,
	},
	{
		"invalid: anonymous func that isn't a wrap",
		func(int) func() { return func() {} },
		endpointFunc,
		lastT, staticT, panicT, nowPastT,
		nil, nil,
		nil, nil,
	},
	{
		"invalid: anonymous func that isn't a wrap #2",
		func(func(), int) {},
		endpointFunc,
		lastT, staticT, panicT, nowPastT,
		nil, nil,
		nil, nil,
	},
	{
		"middleware func",
		func(func(http.ResponseWriter) error, string, bool) (int, error) { return 7, nil },
		middlewareFunc,
		lastF, staticF, panicF, nowPastT,
		[]typeCode{noTypeCode, stringTC, boolTC}, []typeCode{responseWriterTC},
		[]typeCode{intTC, errorTC}, []typeCode{errorTC},
	},
	{
		"middleware func -- all middleware is past static",
		func(func(intType3) (error, intType3), string, bool) (int, error) { return 7, nil },
		middlewareFunc,
		lastF, staticF, panicF, nowPastT,
		[]typeCode{noTypeCode, stringTC, boolTC}, []typeCode{intType3TC},
		[]typeCode{intTC, errorTC}, []typeCode{errorTC, intType3TC},
	},
}

var boolTC = GetTypeCode(reflect.TypeOf((*bool)(nil)).Elem())
var intTC = GetTypeCode(reflect.TypeOf((*int)(nil)).Elem())
var intType3TC = GetTypeCode(reflect.TypeOf((*intType3)(nil)).Elem())
var stringTC = GetTypeCode(reflect.TypeOf((*string)(nil)).Elem())
var terminalErrorTC = GetTypeCode(reflect.TypeOf((*TerminalError)(nil)).Elem())
var requestTC = GetTypeCode(requestType)
var responseWriterTC = GetTypeCode(responseWriterType)
var errorTC = GetTypeCode(errorType)

func recastToTypeCode(i []typeCode) []typeCode {
	// if i == nil { return []typeCode{GetTypeCode(i)} }
	if i == nil {
		return ([]typeCode)(nil)
	}
	return i
}

func een(i []typeCode) string {
	var s []string
	for _, c := range i {
		s = append(s, c.Type().String())
	}
	return "[" + strings.Join(s, "; ") + "]"

	// if len(i) == 0 { return nil }
	// return i
}

// This tests the basic functionality of characterizeFunc()
func TestCharacterize(t *testing.T) {
	t.Parallel()
	for i, test := range characterizeTests {
		f1 := func() {
			t.Logf("trying to characterize... %s", test.name)
			fm, isFuncO := test.fn.(*funcOrigin)
			if isFuncO {
				fm.origin = test.name
				fm.index = i
			} else {
				fm = &funcOrigin{
					origin: test.name,
					index:  i,
					fn:     test.fn,
				}
			}
			tr, _, flows, nowPastStatic := characterizeFuncDetails(fm, true, test.last, test.pastStatic)
			assert.NotNil(t, tr)
			t.Logf("%s type codes: %s vs %s", test.name, test.expectedTypeCode.Type().String(), tr.FuncType.Type().String())
			assert.Equal(t, test.expectedTypeCode.Type().String(), tr.FuncType.Type().String(), "type: "+test.name)
			assert.Equal(t, test.expectedNowPastStatic, nowPastStatic, "past static: "+test.name)
			assert.EqualValues(t, een(test.expectedInputParams), een(flows[inputParams]), "input flow: "+test.name)
			assert.EqualValues(t, een(test.expectedOutputParams), een(flows[outputParams]), "output flow: "+test.name)
			assert.EqualValues(t, een(test.expectedReturnParams), een(flows[returnParams]), "return flow: "+test.name)
			assert.EqualValues(t, een(test.expectedReturnedParams), een(flows[returnedParams]), "returned flow: "+test.name)
		}
		if test.expectedToPanic {
			assert.Panics(t, f1, test.name)
		} else {
			f1()
		}
	}
}

var checkMatchTests = []struct {
	a                      interface{}
	b                      interface{}
	match                  bool
	expectedInputParams    []typeCode
	expectedOutputParams   []typeCode
	expectedReturnParams   []typeCode
	expectedReturnedParams []typeCode
}{
	{
		func() {},
		reflect.TypeOf((*TypeMoreInputs)(nil)).Elem(),
		false,
		nil, nil,
		nil, nil,
	},
}

func toType(i interface{}) reflect.Type {
	if t, already := i.(reflect.Type); already {
		return t
	}
	if c, already := i.(typeCode); already {
		return c.Type()
	}
	return reflect.TypeOf(i)
}

func TestCheckMatch(t *testing.T) {
	t.Parallel()
	for _, test := range checkMatchTests {
		a := toType(test.a)
		b := toType(test.b)
		name := fmt.Sprintf("%s vs %s", a, b)
		t.Logf("trying to test match... %s vs %s", a, b)
		mismatch, flows := checkMatch(a, b)
		if test.match {
			assert.Equal(t, "", mismatch, name)
			assert.EqualValues(t, een(test.expectedInputParams), een(flows[inputParams]), "input flow: "+name)
			assert.EqualValues(t, een(test.expectedOutputParams), een(flows[outputParams]), "output flow: "+name)
			assert.EqualValues(t, een(test.expectedReturnParams), een(flows[returnParams]), "return flow: "+name)
			assert.EqualValues(t, een(test.expectedReturnedParams), een(flows[returnedParams]), "returned flow: "+name)
		} else {
			assert.NotEqual(t, "", mismatch, name)
		}
	}
}

var i int
var i3 intType3
var di = &doesI{}
var dj = &doesJ{}
var interfaceIType = reflect.TypeOf((*interfaceI)(nil)).Elem()
var interfaceJType = reflect.TypeOf((*interfaceJ)(nil)).Elem()
var interfaceKType = reflect.TypeOf((*interfaceK)(nil)).Elem()

var provideSet1 = map[reflect.Type]int{
	requestType:        1,
	responseWriterType: 1,
	reflect.TypeOf(i):  3,
	reflect.TypeOf(i):  4,
}

var provideSet2 = map[reflect.Type]int{
	interfaceIType:     1,
	interfaceJType:     2,
	reflect.TypeOf(di): 3,
	reflect.TypeOf(dj): 4,
	reflect.TypeOf(i):  5,
}

var bestMatchTests = []struct {
	Name    string
	MapData map[reflect.Type]int
	Find    reflect.Type
	Want    reflect.Type
}{
	{
		"responseWriter",
		provideSet1,
		responseWriterType,
		responseWriterType,
	},
	{
		"interface K",
		provideSet2,
		interfaceKType,
		reflect.TypeOf(dj),
	},
}

func TestBestMatch(t *testing.T) {
	t.Parallel()
	for _, test := range bestMatchTests {
		m := make(interfaceMap)
		for typ, layer := range test.MapData {
			t.Logf("%s: #%d get type code for %v", test.Name, layer, typ)
			m.Add(GetTypeCode(typ), layer)
		}
		f := func() {
			for tc, d := range m {
				t.Logf("\tm[%s] = %s (%s) %d", tc.Type(), d.Name, d.TypeCode.Type(), d.Layer)
			}
			got := m.bestMatch(GetTypeCode(test.Find), "searching for "+test.Name)
			assert.Equal(t, test.Want.String(), got.Type().String(), test.Name)
		}
		if test.Want == nil {
			assert.Panics(t, f, test.Name)
		} else {
			f()
		}
	}
}
