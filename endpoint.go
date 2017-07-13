package endpoint

// TODO: tests for CallsInner annotation
// TODO: tests for SideEffects annotation
// TODO: inject path as a Path type.
// TODO: inject route as a mux.Route type.
// TODO: Duplicate service
// TODO: Duplicate endpoint
// TODO: When making copies of valueCollections, do deep copies when they implement a DeepCopy method.
// TODO: Re-use slots in the value collection when values do not overlap in time
// TODO: order the value collection so that middleware can make only partial copies
// TODO: new annotator: skip copying the value collection

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"

	"github.com/gorilla/mux"
)

type EndpointRegistration struct {
	finalFunc  func(http.ResponseWriter, *http.Request)
	initialize func(map[typeCode]interface{})
	path       string
	bound      bool
}

type EndpointRegistrationWithMux struct {
	EndpointRegistration
	muxroutes []func(*mux.Route) *mux.Route
	route     *mux.Route
	err       error
}

type interfaceMap map[typeCode]*interfaceMatchData

type interfaceMatchData struct {
	Name     string
	TypeCode typeCode
	Layer    int
}

type flowMapType map[string][]typeCode

type valueCollection []reflect.Value

type wrapperCollection struct {
	Tr                   *typeRegistration
	Func                 *funcOrigin
	Flows                flowMapType
	WrapWrapper          func(valueCollection, func(valueCollection) valueCollection) valueCollection
	WrapStaticInjector   func(valueCollection)
	WrapFallibleInjector func(valueCollection) (bool, valueCollection)
	WrapEndpoint         func(valueCollection) valueCollection
}

// Start and endpoint: invokes the endpoint and binds it to the
// path.
func (r *EndpointRegistration) start(path string, binder EndpointBinder, preInject map[typeCode]interface{}) {
	r.path = path
	if !r.bound {
		r.initialize(preInject)
		r.bound = true
	}
	binder(path, r.finalFunc)
}

// Start an endpoint: invokes the endpoint and binds it to the  path.   
// If called more than once, subsequent calls to
// EndpointRegistrationWithMux methods that act on the route will
// only act on the last route bound.
func (r *EndpointRegistrationWithMux) start(
	path string,
	binder endpointBinderWithMux,
	preInject map[typeCode]interface{},
) *mux.Route {
	if !r.bound {
		r.initialize(preInject)
		r.bound = true
	}
	r.path = path
	r.route = binder(path, r.finalFunc)
	for _, mod := range r.muxroutes {
		r.route = mod(r.route)
	}
	r.err = r.route.GetError()
	return r.route
}

// CreateEndpoint generates a http.HandlerFunc from a list of handlers.  This bypasses Service,
// ServiceRegistration, ServiceWithMux, and ServiceRegistrationWithMux.  The
// static initializers are invoked immedately.
func CreateEndpoint(funcs ...interface{}) func(http.ResponseWriter, *http.Request) {
	c := newHandlerCollection("createEndpoint", true, funcs...)
	flattened := c.flatten()
	if len(flattened) == 0 {
		panic("at least one handler must be provided")
	}
	reg := buildRegistrationPass2(nil, flattened...)
	reg.initialize(nil)
	return reg.finalFunc
}

// RegisterEndpoint pre-registers an endpoint.  The provided funcs must all match one of the
// following function signatures: HandlerStaticInjectorType, HandlerInjectorType, HandlerMiddlewareType,
// WrapperType and HandlerEndpointType.  The functions provided are invoked in-order.
// Static injectors first and the endpoint last.
//
// The return value does not need to be retained -- it is also remembered
// in the ServiceRegistration.
//
// The endpoint initialization will not run until the service is started.  If the
// service has already been started, the endpoint will be started immediately.
func (s *ServiceRegistration) RegisterEndpoint(path string, funcs ...interface{}) *EndpointRegistration {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.endpoints[path] != nil {
		panic("endpoint path already registered")
	}
	r := buildRegistrationPass1(s.collection, path, funcs...)
	s.endpoints[path] = r
	if s.started != nil {
		r.start(path, s.started.binder, s.collection.injections)
	}
	return r
}

// RegisterEndpoint registers and immedately starts an endpoint.
// The provided funcs must all match one of the
// following function signatures: HandlerStaticInjectorType, HandlerInjectorType, HandlerMiddlewareType,
// WrapperType and HandlerEndpointType.  The functions provided are invoked in-order.
// Static injectors first and the endpoint last.
//
// The return value does not need to be retained -- it is also remembered
// in the Service.
func (s *Service) RegisterEndpoint(path string, funcs ...interface{}) *EndpointRegistration {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.endpoints[path] != nil {
		panic("endpoint path already registered")
	}
	r := buildRegistrationPass1(s.collection, path, funcs...)
	s.endpoints[path] = r
	r.start(path, s.binder, s.collection.injections)
	return r
}

// RegisterEndpoint pre-registers an endpoint.  The provided funcs must all match one of the
// following function signatures: HandlerStaticInjectorType, HandlerInjectorType, HandlerMiddlewareType,
// WrapperType and HandlerEndpointType.  The functions provided are invoked in-order.
// Static injectors first and the endpoint last.
//
// The return value does not need to be retained -- it is also remembered
// in the Service.  The return value can be used to add mux.Route-like
// modifiers.  They will not take effect until the service is started.
//
// The endpoint initialization will not run until the service is started.  If the
// service has already been started, the endpoint will be started immediately.
func (s *ServiceRegistrationWithMux) RegisterEndpoint(path string, funcs ...interface{}) *EndpointRegistrationWithMux {
	s.lock.Lock()
	defer s.lock.Unlock()
	r := buildRegistrationPass1(s.collection, path, funcs...)
	wmux := &EndpointRegistrationWithMux{
		EndpointRegistration: *r,
		muxroutes:            make([]func(*mux.Route) *mux.Route, 0),
	}
	s.endpoints[path] = append(s.endpoints[path], wmux)
	if s.started != nil {
		wmux.start(path, s.started.binder, s.collection.injections)
	}
	return wmux
}

// RegisterEndpoint registers and immediately starts an endpoint.
// The provided funcs must all match one of the
// following function signatures: HandlerStaticInjectorType, HandlerInjectorType, HandlerMiddlewareType,
// WrapperType and HandlerEndpointType.  The functions provided are invoked in-order.
// Static injectors first and the endpoint last.
func (s *ServiceWithMux) RegisterEndpoint(path string, funcs ...interface{}) *mux.Route {
	s.lock.Lock()
	defer s.lock.Unlock()
	r := buildRegistrationPass1(s.collection, path, funcs...)
	wmux := &EndpointRegistrationWithMux{
		EndpointRegistration: *r,
		muxroutes:            make([]func(*mux.Route) *mux.Route, 0),
	}
	s.endpoints[path] = append(s.endpoints[path], wmux)
	return wmux.start(path, s.binder, s.collection.injections)
}

func buildRegistrationPass1(sc *HandlerCollection, path string, funcs ...interface{}) *EndpointRegistration {
	c := newHandlerCollection(path, true, funcs...)
	c = newHandlerCollection("endpoint:"+path, true, sc, c)

	flattened := c.flatten()
	if len(flattened) == 0 {
		panic("at least one handler must be provided")
	}
	return buildRegistrationPass2(c.expectedInjections, flattened...)
}

func buildRegistrationPass2(expectedInjections map[typeCode]bool, funcs ...*funcOrigin) *EndpointRegistration {
	// Pass2: run up and down the list of handlers generate figure out the
	// provided and returned interfaces.

	// Figure out which injectors must be included to provide the inputs necessary for the endpoint
	// and any terminal consumers.

	provide := make(interfaceMap)
	possiblyUsedDown := make(map[typeCode]int)
	for ei, _ := range expectedInjections {
		provide.Add(ei, 1)
	}
	provide.Add(GetTypeCode(responseWriterType), 1)
	possiblyUsedDown[GetTypeCode(responseWriterType)] = -1
	provide.Add(GetTypeCode(requestType), 1)
	possiblyUsedDown[GetTypeCode(requestType)] = -1
	endStaticAt := len(funcs)
	inputsFor := make([]map[typeCode]typeCode, len(funcs))
	for i, fm := range funcs {
		_, _, flows, staticEnd := characterizeFuncDetails(fm, true, i == len(funcs)-1, i >= endStaticAt)
		// fmt.Printf("FLOWS %s: %s\n", fm.describe(), flows.describe())
		if staticEnd && i < endStaticAt {
			endStaticAt = i
		}
		inputsFor[i] = make(map[typeCode]typeCode)
		for _, in := range flows[inputParams] {
			if in != noTypeCode {
				found := provide.bestMatch(in, fm.describe()+" input")
				possiblyUsedDown[found] = i + 1
				inputsFor[i][in] = found
				// fmt.Printf("FILLING %s: input #%d %s being filled with %s\n", fm.describe(), j, in.Type(), found.Type())
			}
		}
		for _, out := range flows[outputParams] {
			if out != noTypeCode {
				provide.Add(out, i+2)
				if possiblyUsedDown[out] == 0 {
					possiblyUsedDown[out] = -1
				}
			}
		}
	}

	includeFunc := make([]bool, len(funcs))
	usedDown := make(map[typeCode]int)
	for tc, _ := range possiblyUsedDown {
		usedDown[tc] = -1
	}
	for i := len(funcs) - 1; i >= 0; i-- {
		fm := funcs[i]
		_, _, flows, _ := characterizeFuncDetails(fm, true, i == len(funcs)-1, i >= endStaticAt)
		include := (i == len(funcs)-1) ||
			(len(flows[outputParams]) == 0) ||
			fm.sideEffects
		if !include {
			for _, out := range flows[outputParams] {
				if usedDown[out] != -1 {
					include = true
					break
				}
			}
		}
		includeFunc[i] = include
		if include {
			for _, tc := range inputsFor[i] {
				if possiblyUsedDown[tc] == -1 {
					fm.panicf("need a %d but can't find one", tc)
				}
				usedDown[tc] = possiblyUsedDown[tc]
			}
		}
	}

	// TODO: order this so that vmap copies can be partial copies
	downVmap := make(map[typeCode]int)
	downCount := 0
	for tc, where := range usedDown {
		if where == -1 {
			downVmap[tc] = -1
		} else {
			downVmap[tc] = downCount
			downCount++
		}
	}

	// Validate the returned values are consumed

	returns := make(interfaceMap)
	usedUp := make(map[typeCode]int)
	usedUpOrigin := make(map[typeCode]int)
	returnedTo := make([]map[typeCode]typeCode, len(funcs))
	leftmostMiddleware := len(funcs)
	mustZeroIfInnerNotCalled := make([][]typeCode, len(funcs))
	for i := len(funcs) - 1; i >= 0; i-- {
		fm := funcs[i]
		fchar, _, flows, _ := characterizeFuncDetails(fm, true, i == len(funcs)-1, i >= endStaticAt)
		returnedTo[i] = make(map[typeCode]typeCode)
		mustZeroIfInnerNotCalled[i] = make([]typeCode, 0)
		for _, comingUp := range flows[returnedParams] {
			found := returns.bestMatch(comingUp, "to be returned to "+fm.describe())
			usedUp[found] = len(funcs) - i + 1
			returnedTo[i][comingUp] = found

			if usedUpOrigin[found] > leftmostMiddleware {
				mustZeroIfInnerNotCalled[leftmostMiddleware] = append(
					mustZeroIfInnerNotCalled[leftmostMiddleware], found)
			}
		}
		for _, o := range flows[returnParams] {
			returns.Add(o, len(funcs)-i+2)
			usedUp[o] = -1
			usedUpOrigin[o] = i
		}
		if fchar.Group == middlewareGroup {
			leftmostMiddleware = i
		}
	}
	upVmap := make(map[typeCode]int)
	upCount := 0
	for tc, usedIndex := range usedUp {
		if usedIndex == -1 {
			funcs[usedUpOrigin[tc]].panicf("value returned (%s) is never used", tc.Type())
		} else {
			upVmap[tc] = upCount
			upCount++
		}
	}

	// Generate wrappers and split the handlers into groups (static, middleware, endpoint)

	collections := make(map[string][]wrapperCollection)
	for i, fm := range funcs {
		if !includeFunc[i] {
			continue
		}
		wrapped := generateWrappers(fm, inputsFor[i], returnedTo[i], downVmap, upVmap, upCount, mustZeroIfInnerNotCalled[i], i == len(funcs)-1, i >= endStaticAt)
		// fmt.Printf("%s: %s/%s\n", fm.describe(), wrapped.Tr.Group, wrapped.Tr.Class.Type())

		collections[wrapped.Tr.Group] = append(collections[wrapped.Tr.Group], wrapped)
	}

	return buildRegistrationPass3(collections, expectedInjections, downCount, upCount, downVmap)
}

func buildRegistrationPass3(
	collections map[string][]wrapperCollection,
	expectedInjections map[typeCode]bool,
	downCount int,
	upCount int,
	downVmap map[typeCode]int,
) *EndpointRegistration {
	if len(collections[endpointGroup]) != 1 {
		panic("should have one endpoint")
	}

	f := collections[endpointGroup][0].WrapEndpoint
	for i := len(collections[middlewareGroup]) - 1; i >= 0; i-- {
		n := collections[middlewareGroup][i]

		switch n.Tr.Class {
		case middlewareFunc:
			inner := f
			w := n.WrapWrapper
			f = func(v valueCollection) valueCollection {
				return w(v, inner)
			}
		case injectorFunc, fallibleInjectorFunc:
			// j is the index of the leftmost injector
			// fmt.Printf("i=%d, injectorFunc=%v, fallibleInjectorFunc=%v\n", i, injectorFunc, fallibleInjectorFunc)
			j := i - 1
		Injectors:
			for j >= 0 {
				// fmt.Printf("class collection[%d] = %v\n", j, collections[middlewareGroup][j].Tr.Class)
				switch collections[middlewareGroup][j].Tr.Class {
				default:
					break Injectors
				case injectorFunc, fallibleInjectorFunc: //okay
				}
				j--
			}
			j++
			// fmt.Printf("Combining %d injectors into one loop (j=%d)\n", i-j+1, j)
			next := f
			injectors := make([]func(valueCollection) (bool, valueCollection), 0, i-j+1)
			for k := j; k <= i; k++ {
				injectors = append(injectors, collections[middlewareGroup][k].WrapFallibleInjector)
				// fmt.Printf("inject %s = %v\n", collections[middlewareGroup][k].Func.describe(), injectors[len(injectors)-1])
			}
			f = func(v valueCollection) valueCollection {
				for _, injector := range injectors {
					errored, upV := injector(v)
					if errored {
						return upV
					}
				}
				return next(v)
			}
			i = j
		default:
			panic("should not be here")
		}
	}

	baseValues := make(valueCollection, downCount)
	initDone := false
	var initLock sync.Mutex
	initFunc := func(injections map[typeCode]interface{}) {
		initLock.Lock()
		defer initLock.Unlock()
		if !initDone {
			for ei, _ := range expectedInjections {
				i, used := downVmap[ei]
				if used {
					v, provided := injections[ei]
					if !provided {
						panic(fmt.Sprintf("Expected a %s to be provided through Inject() but it didn't happen", ei.Type().String()))
					}
					baseValues[i] = reflect.ValueOf(v)
				}
			}
			for _, inj := range collections[staticGroup] {
				inj.WrapStaticInjector(baseValues)
			}
			initDone = true
		}
	}

	wi, found := downVmap[GetTypeCode(responseWriterType)]
	if !found {
		panic("no response writer in vmap")
	}
	ri, found := downVmap[GetTypeCode(requestType)]
	if !found {
		panic("no request in vmap")
	}
	finalFunc := func(w http.ResponseWriter, r *http.Request) {
		values := baseValues.Copy()
		if wi != -1 {
			values[wi] = reflect.ValueOf(w)
		}
		if ri != -1 {
			values[ri] = reflect.ValueOf(r)
		}
		_ = f(values)
	}

	return &EndpointRegistration{
		initialize: initFunc,
		finalFunc:  finalFunc,
	}
}

func (m interfaceMap) Add(t typeCode, layer int) {
	m[t] = &interfaceMatchData{
		Name:     t.Type().String(),
		TypeCode: t,
		Layer:    layer,
	}
}

func (m interfaceMap) Copy() interfaceMap {
	copy := make(interfaceMap)
	for tc, imd := range m {
		copy[tc] = imd
	}
	return copy
}

func aGreaterBInts(a []int, b []int) bool {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] > b[i] {
			return true
		}
		if a[i] < b[i] {
			return false
		}
	}
	if len(a) < len(b) {
		return false
	}
	return true
}

func (m interfaceMap) bestMatch(match typeCode, purpose string) typeCode {
	_, found := m[match]
	if found {
		return match
	}
	if match.Type().Kind() != reflect.Interface {
		panic(fmt.Sprintf("No exact match found for %s looking for %s", match.Type(), purpose))
	}
	// What is the best match?
	// (*) Highest layer number
	// (*) Same package path for source and desitnation
	// (*) Highest method count
	// (*) Lowest typeCode value
	var best struct {
		tc    typeCode
		imd   *interfaceMatchData
		score []int
	}
	score := func(tc typeCode, imd *interfaceMatchData) []int {
		samePathScore := 0
		if imd.TypeCode.Type().PkgPath() == match.Type().PkgPath() {
			samePathScore = 1
		}
		return []int{imd.Layer, samePathScore, imd.TypeCode.Type().NumMethod(), int(tc)}
	}
	for tc, imd := range m {
		if !imd.TypeCode.Type().Implements(match.Type()) {
			continue
		}
		s := score(tc, imd)
		if best.imd == nil || aGreaterBInts(s, best.score) {
			best.tc = tc
			best.imd = imd
			best.score = s
			continue
		}
	}
	if best.imd == nil {
		panic(fmt.Sprintf("No match found for %s needed %s", match.Type(), purpose))
	}
	return best.tc
}

func (m flowMapType) describe() string {
	return fmt.Sprintf("takes [%s] returns(up) [%s] sends(down) [%s] receives [%s]",
		codeListString(m[inputParams]),
		codeListString(m[returnParams]),
		codeListString(m[outputParams]),
		codeListString(m[returnedParams]),
	)
}

func codeListString(codes []typeCode) string {
	var s []string
	for _, tc := range codes {
		s = append(s, tc.Type().String())
	}
	return strings.Join(s, ", ")
}

func characterizeFunc(fm *funcOrigin, panicOnFailure bool, last bool, alreadyPastStatic bool) *typeRegistration {
	c, _, _, _ := characterizeFuncDetails(fm, panicOnFailure, last, alreadyPastStatic)
	return c
}

func characterizeFuncDetails(fm *funcOrigin, panicOnFalure bool, last bool, alreadyPastStatic bool) (
	tr *typeRegistration,
	t reflect.Type,
	flows flowMapType,
	nowPastStatic bool,
) {
	t = reflect.TypeOf(fm.fn)
	rejectReasons := make([]string, len(primaryRegistrations))
	nowPastStatic = alreadyPastStatic
	var i int
RegistrationCheck:
	for i, tr = range primaryRegistrations {
		rejectReasons[i] = tr.Type.String() + ": "
		if last && tr.NotLast {
			rejectReasons[i] += "last"
			continue RegistrationCheck
		}
		if !last && tr.Last {
			rejectReasons[i] += "not last"
			continue RegistrationCheck
		}
		if fm.pastStatic && tr.StaticOnly {
			rejectReasons[i] += "marked not static"
			continue RegistrationCheck
		}
		if t.Kind() == reflect.Func && t.NumOut() == 0 && tr.StaticOnly {
			rejectReasons[i] += "pure consumes are not static"
			continue RegistrationCheck
		}
		if alreadyPastStatic && tr.StaticOnly {
			rejectReasons[i] += "past statics"
			continue RegistrationCheck
		}

		var mismatch string
		mismatch, flows = checkMatch(t, tr.Type)

		if mismatch != "" {
			rejectReasons[i] += mismatch
			continue
		}
		if !alreadyPastStatic {
		StaticCheck:
			for _, flow := range flows {
				for _, tc := range flow {
					for _, notStaticType := range allNotInStatic {
						m, _ := checkMatch(tc.Type(), notStaticType)
						if m == "" {
							nowPastStatic = true
							if tr.StaticOnly {
								rejectReasons[i] += "parameter not static"
								continue RegistrationCheck
							}
							break StaticCheck
						}
					}
				}
			}
		}
		if tr.PastStatic {
			nowPastStatic = true
		}
		return
	}
	if panicOnFalure {
		panic(fmt.Sprintf("Could not match type of function %s to any prototype: %s", fm.describe(), strings.Join(rejectReasons, "; ")))
	}
	tr = nil
	return
}

// Compares two functions (usually functions)
//
// mismatch: empty if "have" can be wrapped up to be "want".  An explanation of
// why "have" won't work as "want" if "have" cannot be wrapped.
//
func checkMatch(
	have reflect.Type,
	want reflect.Type,
) (
	mismatch string,
	flows flowMapType,
) {
	tr := getRegistration(want)
	switch want.Kind() {
	case reflect.Func:
		if have.Kind() != reflect.Func {
			mismatch = "not func"
			return
		}
		// do not return ... more tests follow
	case reflect.Interface:
		if tr.NoAnonymous && have.Kind() == reflect.Func && have.PkgPath() == "" {
			mismatch = "anonymous func not allowed"
		}
		if !have.Implements(want) {
			mismatch = "does not implement"
		}
		return
	case reflect.Ptr:
		if have.Kind() == reflect.Ptr {
			mismatch, flows = checkMatch(have.Elem(), want.Elem())
		} else {
			mismatch = "not a pointer"
		}
		return
	default:
		if !(have.AssignableTo(want) || have.ConvertibleTo(want)) {
			mismatch = "could not assign or convert"
		}
		return
	}
	flows = make(flowMapType)

	// fmt.Printf("Adding takes:%s and returns:%s for %s\n", tr.Takes, tr.Returns, have)
	if tr.Takes != "" {
		flows[tr.Takes] = make([]typeCode, 0, have.NumIn())
		for i := 0; i < have.NumIn(); i++ {
			flows[tr.Takes] = append(flows[tr.Takes], GetTypeCode(have.In(i)))
		}
	}

	if tr.Returns != "" {
		flows[tr.Returns] = make([]typeCode, 0, have.NumOut())
		for i := 0; i < have.NumOut(); i++ {
			flows[tr.Returns] = append(flows[tr.Returns], GetTypeCode(have.Out(i)))
		}
	}

	// This is a greedy matcher.  If there is more than one repeat interface{} only the
	// first one will get anything.
	checkParamList := func(inFlow string, hn int, h func(int) reflect.Type, wn int, w func(int) reflect.Type) string {
		hi := 0
		wi := 0
		wiRepeating := -1
		extras := -1
		for hi < hn && wi < wn {
			wtr := getRegistration(w(wi))
			// TODO: if w(wi).Kind == reflect.Func

			mismatch, subflows := checkMatch(h(hi), w(wi))

			if mismatch == "" {
				if wtr.NoFlow && inFlow != "" {
					flows[inFlow][hi] = noTypeCode
				}
				if wtr.CanSkip && extras == -1 {
					extras = hi
				}
				hi++
				if wtr.CanRepeat {
					wiRepeating = wi
				} else {
					wi++
				}
				for f, l := range subflows {
					if flows[f] == nil {
						flows[f] = l
					} else {
						panic(fmt.Sprintf("multiple instances of the same sub-flow (%s) when matching against %s", f, want))
					}
				}
			} else if wtr.CanSkip {
				wi++
			} else if wiRepeating == wi {
				wi++
			} else {
				return fmt.Sprintf("have %d %s not okay as want %d %s: %s", hi, h(hi), wi, w(wi), mismatch)
			}
		}
		if hi < hn {
			return "has extra parameters"
		}
		for wi < wn {
			wtr := getRegistration(w(wi))
			if wtr.CanSkip {
				wi++
			} else {
				return "too few parameters"
			}
		}
		return ""
	}

	mm := checkParamList(tr.Takes, have.NumIn(), have.In, want.NumIn(), want.In)
	if mm != "" {
		mismatch = "inputs: " + mm
		return
	}

	mm = checkParamList(tr.Returns, have.NumOut(), have.Out, want.NumOut(), want.Out)
	if mm != "" {
		mismatch = "outputs: " + mm
		return
	}

	if tr.Anonymous && have.Name() != "" {
		mismatch = "expecting have to be anonymous"
	}

	// Overrides
	switch tr.FuncType {
	case fallibleInjectorFunc:
		flows[returnParams] = []typeCode{GetTypeCode(errorType)}
		flows[outputParams] = flows[outputParams][1:]
	}

	return
}

func okayAs(have reflect.Type, want reflect.Type) bool {
	if have == want {
		return true
	}
	mismatch, _ := checkMatch(have, want)
	return mismatch == ""
}

func (v valueCollection) Copy() valueCollection {
	copy := make(valueCollection, len(v))
	for i, val := range v {
		copy[i] = val
	}
	return copy
}

func generateWrappers(
	fm *funcOrigin,
	downRmap map[typeCode]typeCode,
	upRmap map[typeCode]typeCode,
	downVmap map[typeCode]int,
	upVmap map[typeCode]int,
	upCount int,
	mustZeroIfSkipInner []typeCode,
	last bool,
	pastStatic bool,
) (
	wrappers wrapperCollection,
) {
	tr, t, flows, _ := characterizeFuncDetails(fm, true, last, pastStatic)
	fv := reflect.ValueOf(fm.fn)
	wrappers.Tr = tr
	wrappers.Flows = flows
	wrappers.Func = fm

	// fmt.Printf("generating for %s\n", t)

	flowMap := func(param string, start int, rmap map[typeCode]typeCode, vmap map[typeCode]int, purpose string) ([]int, int, []reflect.Type) {
		flow, found := flows[param]
		if !found {
			panic("oops")
		}
		m := make([]int, len(flow))
		types := make([]reflect.Type, len(flow))
		for i, p := range flow {
			if i < start && p != noTypeCode {
				fm.panicf("expecting notype for skipped %s, %d %s", param, i, purpose)
			}
			if p == noTypeCode && i >= start {
				fm.panicf("expecting skip for notype %s, %d %s", param, i, purpose)
			}
			if p != noTypeCode {
				var useP typeCode
				if rmap == nil {
					useP = p
				} else {
					var found bool
					useP, found = rmap[p]
					if !found {
						fm.panicf("typemap incomplete %s %d %s missing %s", param, i, purpose, p.Type())
					}
				}
				vci, found := vmap[useP]
				if !found {
					fm.panicf("vmap incomplete %s %d %s missing %s was %s", param, i, purpose, useP.Type(), p.Type())
				}
				m[i] = vci
				types[i] = useP.Type()
			}
		}
		// fmt.Printf("\t%s map %s: %d items\n", purpose, param, len(flow))
		return m, len(flow), types
	}

	inputMapper := func(start int, param string, rmap map[typeCode]typeCode, vmap map[typeCode]int, purpose string) func(valueCollection) []reflect.Value {
		m, numIn, types := flowMap(param, start, rmap, vmap, param+" valueCollection->[]")
		for i := start; i < numIn; i++ {
			if m[i] == -1 {
				fm.panicf("unexpected discarded value %s %d", param, i)
			}
		}
		return func(v valueCollection) []reflect.Value {
			in := make([]reflect.Value, numIn)
			for i := start; i < numIn; i++ {
				in[i] = v[m[i]]
				if !in[i].IsValid() {
					in[i] = reflect.Zero(types[i])
				}
			}
			return in
		}
	}

	outputMapper := func(start int, param string, vmap map[typeCode]int, purpose string) func(valueCollection, []reflect.Value) {
		m, numOut, types := flowMap(param, start, nil, vmap, param+" []->valueCollection")
		return func(v valueCollection, out []reflect.Value) {
			for i := start; i < numOut; i++ {
				if m[i] != -1 {
					v[m[i]] = out[i]
					if !v[m[i]].IsValid() {
						v[m[i]] = reflect.Zero(types[i])
					}
				}
			}
		}
	}

	makeZero := func() func() valueCollection {
		zeroMap := make(map[int]reflect.Type)
		newMap := make(map[int]reflect.Type)
		for _, p := range mustZeroIfSkipInner {
			i, found := upVmap[p]
			if !found {
				fm.panicf("no type mapping for %s that must be zeroed", p.Type())
			}
			if reflect.Zero(p.Type()).CanInterface() {
				zeroMap[i] = p.Type()
			} else if reflect.New(p.Type()).CanAddr() && reflect.New(p.Type()).Elem().CanInterface() {
				newMap[i] = p.Type()
			} else if !fm.callsInner {
				fm.panicf("cannot create useful zero value for %s (needed if inner() doesn't get called)", p.Type())
			}
		}
		return func() valueCollection {
			zeroUpV := make(valueCollection, upCount)
			for i, typ := range zeroMap {
				zeroUpV[i] = reflect.Zero(typ)
			}
			for i, typ := range newMap {
				zeroUpV[i] = reflect.New(typ).Elem()
			}
			return zeroUpV
		}
	}

	switch tr.Class {
	case endpointFunc:
		inMap := inputMapper(0, inputParams, downRmap, downVmap, "in")
		upMap := outputMapper(0, returnParams, upVmap, "up")
		wrappers.WrapEndpoint = func(downV valueCollection) valueCollection {
			in := inMap(downV)
			upV := make(valueCollection, upCount)
			upMap(upV, fv.Call(in))
			return upV
		}
	case middlewareFunc:
		inMap := inputMapper(1, inputParams, downRmap, downVmap, "in")  // parmeters to the middleware handler
		outMap := outputMapper(0, outputParams, downVmap, "out")        // parameters to inner()
		upMap := outputMapper(0, returnParams, upVmap, "up")            // return values from middleward handler
		retMap := inputMapper(0, returnedParams, upRmap, upVmap, "ret") // return values from inner()
		zero := makeZero()
		wrappers.WrapWrapper = func(downV valueCollection, next func(valueCollection) valueCollection) valueCollection {
			var upV valueCollection
			downVCopy := downV.Copy()
			callCount := 0

			rTypes := make([]reflect.Type, len(flows[returnedParams]))
			for i, tc := range flows[returnedParams] {
				rTypes[i] = tc.Type()
			}

			// this is not built outside WrapWrapper for thread safty
			inner := func(i []reflect.Value) []reflect.Value {
				if callCount > 0 {
					for i, val := range downVCopy {
						downV[i] = val
					}
				}
				callCount++
				outMap(downV, i)
				upV = next(downV)
				r := retMap(upV)
				for i, v := range r {
					if rTypes[i].Kind() == reflect.Interface {
						r[i] = v.Convert(rTypes[i])
					}
				}
				return r
			}
			in := inMap(downV)
			in[0] = reflect.MakeFunc(t.In(0), inner)
			out := fv.Call(in)
			if callCount == 0 {
				upV = zero()
			}
			upMap(upV, out)
			return upV
		}
	case fallibleInjectorFunc:
		inMap := inputMapper(0, inputParams, downRmap, downVmap, "in")
		outMap := outputMapper(0, outputParams, downVmap, "out")
		upMap := outputMapper(0, returnParams, upVmap, "up")
		zero := makeZero()
		wrappers.WrapFallibleInjector = func(v valueCollection) (bool, valueCollection) {
			in := inMap(v)
			out := fv.Call(in)
			if out[0].Interface() != nil {
				upV := zero()
				upMap(upV, out[0:1])
				return true, upV
			}
			outMap(v, out[1:])
			return false, nil
		}
	case injectorFunc:
		inMap := inputMapper(0, inputParams, downRmap, downVmap, "in")
		outMap := outputMapper(0, outputParams, downVmap, "out")
		wrappers.WrapFallibleInjector = func(v valueCollection) (bool, valueCollection) {
			in := inMap(v)
			outMap(v, fv.Call(in))
			return false, nil
		}
	case staticInjectorFunc:
		inMap := inputMapper(0, inputParams, downRmap, downVmap, "in")
		outMap := outputMapper(0, outputParams, downVmap, "out")
		wrappers.WrapStaticInjector = func(v valueCollection) {
			in := inMap(v)
			outMap(v, fv.Call(in))
		}
	default:
		panic("should not be here")
	}
	return
}
