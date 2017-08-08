package endpoint

import (
	"fmt"
	"net/http"
	"reflect"
	"sync"

	"github.com/gorilla/mux"
)

// HandlerCollection provides a way to bundle up several handlers
// so that they can be refered to together as a set.
// A HanlderCollection can be used as a handler func in any place
// that a handler func can be used including NewHandlerCollection(),
// PreRegisterService(), and PreRegisterEndpoint.
type HandlerCollection struct {
	name               string
	collections        map[string][]*funcOrigin
	expectedInjections map[typeCode]bool
	injections         map[typeCode]interface{}
}

// AnnotateNotStatic wraps any handler func argument and
// marks that handler as not being a static injector.
// Since handlers will be invoked in order, this also implies
// that nothing that follows this handler is a static injector.
//
// For example:
//
//	var InjectCurrentTime = AnnotateNotStatic(func() time.Time {
//		return time.Now()
//	})
//
// EXPERIMENTAL FEATURE. MAY BE REMOVED.
func AnnotateNotStatic(handlerOrCollection interface{}) interface{} {
	return annotateNotStatic(handlerOrCollection)
}

var annotateNotStatic = makeAnnotate(func(fm *funcOrigin) {
	fm.pastStatic = true
})

// AnnotateSideEffects wraps any handler func argument
// and marks that handler as having side effects.  Normally
// an injector that produces at least one type and none of the
// types that the injector produce are consumed dropped from the
// dropped from the handler list.  If AnnotateSideEffects() is
// used to wrap that injector, then the injector will be invoked
// even though its output is not used.
//
// For example:
//
//	var counter = 0
//	var counterLock sync.Mutex
//	type InvocationCounter int
//	var CountEndpointInvocations = AnnotateSideEffects(func() InvocationCounter {
//		counterLock.Lock()
//		counter++
//		counterLock.Unlock()
//		return InvocationCounter(counter)
//	})
//
// EXPERIMENTAL FEATURE. MAY BE REMOVED.
func AnnotateSideEffects(handlerOrCollection interface{}) interface{} {
	return annotateSideEffects(handlerOrCollection)
}

var annotateSideEffects = makeAnnotate(func(fm *funcOrigin) {
	fm.sideEffects = true
})

// AnnotateCallsInner wraps middleware handlers and marks that the
// middleware handler as guaranteed to call
// inner().  This is important only in the case when a downstream
// handler is returning a value consumed by an upstream middleware
// handler and that value has no zero value that reflect can generate
// in the event that inner() is not called.
// Without AnnotateCallsInner(), some handler  sequences may be rejected
// due to being unable to create zero values.
//
// Using AnnotateCallsInner on other handlers is a no-op.
//
// EXPERIMENTAL FEATURE. MAY BE REMOVED.
func AnnotateCallsInner(handlerOrCollection interface{}) interface{} {
	return annotateCallsInner(handlerOrCollection)
}

var annotateCallsInner = makeAnnotate(func(fm *funcOrigin) {
	fm.callsInner = true
})

// Service allows a group of related endpoints to be started
// together. This form of service represents an already-started
// service that binds its enpoints using a simple binder like
// http.ServeMux.HandleFunc().
type Service struct {
	Name       string
	endpoints  map[string]*EndpointRegistration
	collection *HandlerCollection
	binder     EndpointBinder
	lock       sync.Mutex
}

// ServiceRegistration allows a group of related endpoints to be started
// together. This form of service represents pre-registered service
// service that binds its enpoints using a simple binder like
// http.ServeMux.HandleFunc().  None of the endpoints associated
// with this service will initialize themsleves or start listening
// until Start() is called.
type ServiceRegistration struct {
	Name       string
	started    *Service
	endpoints  map[string]*EndpointRegistration
	collection *HandlerCollection
	lock       sync.Mutex
}

// ServiceWithMux allows a group of related endpoints to be started
// together. This form of service represents an already-started
// service that binds its enpoints using gorilla
// mux.Router.HandleFunc.
type ServiceWithMux struct {
	Name       string
	endpoints  map[string][]*EndpointRegistrationWithMux
	collection *HandlerCollection
	binder     endpointBinderWithMux
	lock       sync.Mutex
}

// ServiceRegistrationWithMux allows a group of related endpoints to be started
// together. This form of service represents pre-registered service
// service that binds its enpoints using gorilla
// mux.Router.HandleFunc.  None of the endpoints associated
// with this service will initialize themsleves or start listening
// until Start() is called.
type ServiceRegistrationWithMux struct {
	Name       string
	started    *ServiceWithMux
	endpoints  map[string][]*EndpointRegistrationWithMux
	collection *HandlerCollection
	lock       sync.Mutex
}

type funcOrigin struct {
	origin      string
	index       int
	fn          interface{}
	sideEffects bool
	pastStatic  bool
	callsInner  bool
}

func (fm *funcOrigin) Copy() *funcOrigin {
	return &funcOrigin{
		origin:      fm.origin,
		index:       fm.index,
		fn:          fm.index,
		sideEffects: fm.sideEffects,
		pastStatic:  fm.pastStatic,
		callsInner:  fm.callsInner,
	}
}

type injectionWish typeCode

// AutoInject makes a promise that values will be provided
// to the service via service.Inject() before the service is started.  This is done
// by either providing an example of the type or by providing a reflect.Type.
// The value provided is not retained, just the type of the value provided.
//
// This is only relevant for pre-registered services.
//
// EXPERIMENTAL FEATURE. MAY BE REMOVED.
func AutoInject(v interface{}) injectionWish {
	t, isType := v.(reflect.Type)
	if !isType {
		t = reflect.TypeOf(v)
	}
	return injectionWish(GetTypeCode(t))
}

// NewHandlerCollection creates a collection of handlers.
//
// The name of the collection is used for error messages and is otherwise
// irrelevant.
//
// The passed in funcs must match one of the handler types.
//
// When combining handler collections and lists of handlers, static injectors
// may be pulled from handler collections and treated as static injectors even if
// the handler collection as a whole comes after a regular injector.
func NewHandlerCollection(name string, funcs ...interface{}) *HandlerCollection {
	return newHandlerCollection(name, false, funcs...)
}

// NewHandlerCollectionWithEndpoint creates a handler collection that includes an endpoint.
func NewHandlerCollectionWithEndpoint(name string, funcs ...interface{}) *HandlerCollection {
	return newHandlerCollection(name, true, funcs...)
}

func (c *HandlerCollection) flatten() []*funcOrigin {
	var flattened []*funcOrigin
	for _, g := range allGroups {
		flattened = append(flattened, c.collections[g]...)
	}

	// for i, f := range flattened { fmt.Printf("EP1: %d: %s\n", i, f.describe()) }
	return flattened
}

func newHandlerCollection(name string, hasEndpoint bool, funcs ...interface{}) *HandlerCollection {
	// fmt.Printf("NEW HANDLER COLLECTION -------------------------------------------------\n")
	nonStaticTypes := make(map[typeCode]bool)
	c := &HandlerCollection{
		name:               name,
		collections:        make(map[string][]*funcOrigin),
		expectedInjections: make(map[typeCode]bool),
		injections:         make(map[typeCode]interface{}),
	}
	endOfStaticSeen := false

	addFunc := func(endOfStaticSeen bool, lastGroup bool, fn interface{}) bool {
		fm, isFuncO := fn.(*funcOrigin)
		if isFuncO {
			// Previously annotated
			fm := fm.Copy()
			fm.origin = name
			fm.index = i
		} else {
			fm = &funcOrigin{
				origin: name,
				index:  i,
				fn:     fn,
			}
		}

		fchar, ftype, flows, nowPastStatic := characterizeFuncDetails(fm, false, hasEndpoint && i == len(funcs)-1 && lastGroup, endOfStaticSeen)
		if fchar == nil {
			fm.panicf("could not characterize parameter %s-#%d (%s) as a valid handler", name, i, ftype)
		}
		// fmt.Printf("%s: GROUP %s\n" , fm.describe(), fchar.Group)
		endOfStaticSeen = endOfStaticSeen || nowPastStatic
		if fchar.Group == endpointGroup {
			if !hasEndpoint {
				fm.panicf("internal error: should not interpret anything as an endpoint at the service level")
			}
			if len(c.collections[fchar.Group]) > 0 {
				panic("only one endpoint allowed")
			}
		}
		if fchar.Group == staticGroup {
			for _, in := range flows[inputParams] {
				if nonStaticTypes[in] {
					endOfStaticSeen = true
					fchar, ftype, flows, nowPastStatic = characterizeFuncDetails(fm, false, hasEndpoint && i == len(funcs)-1 && lastGroup, endOfStaticSeen)
					break
				}
			}
		}
		if endOfStaticSeen {
			for _, out := range flows[outputParams] {
				nonStaticTypes[out] = true
			}
		}
		c.collections[fchar.Group] = append(c.collections[fchar.Group], fm)
		fm.pastStatic = endOfStaticSeen
		return endOfStaticSeen
	}

	for i, fn := range funcs {
		subcollection, isSubcollection := fn.(*HandlerCollection)
		if isSubcollection {
			if subcollection == nil {
				continue
			}
			if len(subcollection.collections[endpointGroup]) > 0 {
				if !hasEndpoint || i != len(funcs)-1 {
					panic("should not have any endpoints in sub-HandlerCollections")
				}
				if len(c.collections[endpointGroup]) > 0 {
					panic("only one endpoint allowed")
				}
			}
			endOfStatic := false
			for _, fn := range subcollection.collections[staticGroup] {
				endOfStatic = addFunc(endOfStatic, false, fn)
			}
			for _, fn := range subcollection.collections[middlewareGroup] {
				endOfStatic = addFunc(endOfStatic, false, fn)
			}
			c.collections[endpointGroup] = append(c.collections[endpointGroup], subcollection.collections[endpointGroup]...)

			for iw, _ := range subcollection.expectedInjections {
				c.expectedInjections[iw] = true
			}
			continue
		}

		injectionWish, isWish := fn.(injectionWish)
		if isWish {
			c.expectedInjections[typeCode(injectionWish)] = true
			continue
		}

		endOfStaticSeen = addFunc(endOfStaticSeen, true, fn)
	}
	return c
}

func (c *HandlerCollection) forEach(fn func(*funcOrigin)) {
	for _, set := range c.collections {
		for _, fm := range set {
			fn(fm)
		}
	}
}

// PreRegisterService creates a service that must be Start()ed later.
//
// The passed in funcs follow the same rules as for the funcs in a
// HandlerCollection.
//
// The injectors and middlware functions will preceed any injectors
// and middleware specified on each endpoint that registeres with this
// service.
//
// PreRegsteredServices do not initialize or bind to handlers until
// they are Start()ed.
//
// The name of the service is just used for error messages and is otherwise ignored.
func PreRegisterService(name string, funcs ...interface{}) *ServiceRegistration {
	return registerService(name, funcs...)
}

// RegisterService creates a service and starts it immediately.
func RegisterService(name string, binder EndpointBinder, funcs ...interface{}) *Service {
	sr := PreRegisterService(name, funcs...)
	return sr.Start(binder)
}

// PreRegisterServiceWithMux creates a service that must be Start()ed later.
//
// The passed in funcs follow the same rules as for the funcs in a
// HandlerCollection.
//
// The injectors and middlware functions will preceed any injectors
// and middleware specified on each endpoint that registeres with this
// service.
//
// PreRegsteredServices do not initialize or bind to handlers until
// they are Start()ed.
//
// The name of the service is just used for error messages and is otherwise ignored.
func PreRegisterServiceWithMux(name string, funcs ...interface{}) *ServiceRegistrationWithMux {
	return registerServiceWithMux(name, funcs...)
}

// RegisterServiceWithMux creates a service and starts it immediately.
func RegisterServiceWithMux(name string, router *mux.Router, funcs ...interface{}) *ServiceWithMux {
	sr := PreRegisterServiceWithMux(name, funcs...)
	return sr.Start(router)
}

func (fm *funcOrigin) describe() string {
	var t string
	if fm.fn == nil {
		t = "nil"
	} else {
		t = reflect.TypeOf(fm.fn).String()
	}
	return fmt.Sprintf("%s[%d] %s", fm.origin, fm.index, t)
}

func (fm *funcOrigin) panicf(format string, args ...interface{}) {
	panic(fm.describe() + ": " + fmt.Sprintf(format, args...))
}

func registerService(name string, funcs ...interface{}) *ServiceRegistration {
	return &ServiceRegistration{
		Name:       name,
		endpoints:  make(map[string]*EndpointRegistration),
		collection: NewHandlerCollection(name, funcs...),
	}
}

func registerServiceWithMux(name string, funcs ...interface{}) *ServiceRegistrationWithMux {
	return &ServiceRegistrationWithMux{
		Name:       name,
		endpoints:  make(map[string][]*EndpointRegistrationWithMux),
		collection: NewHandlerCollection(name, funcs...),
	}
}

// EndpointBinder is the signature of the binding function
// used to start a ServiceRegistration.
type EndpointBinder func(path string, fn http.HandlerFunc)

// Start runs all staticInjectors for all endpoints pre-registered with this
// service.  Bind all endpoints and starts listening.   Start() may
// only be called once.
func (s *ServiceRegistration) Start(binder EndpointBinder) *Service {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.started != nil {
		panic("duplicate call to Start()")
	}
	for path, endpoint := range s.endpoints {
		endpoint.start(path, binder, s.collection.injections)
	}
	svc := &Service{
		Name:       s.Name,
		endpoints:  s.endpoints,
		collection: s.collection,
		binder:     binder,
	}
	s.started = svc
	return svc
}

// Inject provides a value for one of the types promised by AutoInject.
func (s *ServiceRegistration) Inject(v interface{}) *ServiceRegistration {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.collection.injections[GetTypeCode(reflect.TypeOf(v))] = v
	return s
}

type endpointBinderWithMux func(string, func(http.ResponseWriter, *http.Request)) *mux.Route

// Inject provides a value for one of the types promised by AutoInject.
func (s *ServiceRegistrationWithMux) Inject(v interface{}) *ServiceRegistrationWithMux {
	s.collection.injections[GetTypeCode(reflect.TypeOf(v))] = v
	return s
}

// Start calls endpoints initializers for this Service and then registers all the
// endpoint handlers to the router.   Start() should be called at most once.
func (s *ServiceRegistrationWithMux) Start(router *mux.Router) *ServiceWithMux {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.started != nil {
		panic("duplicate call to Start()")
	}
	for path, el := range s.endpoints {
		for _, endpoint := range el {
			endpoint.start(path, router.HandleFunc, s.collection.injections)
		}
	}
	svc := &ServiceWithMux{
		Name:       s.Name,
		endpoints:  s.endpoints,
		collection: s.collection,
		binder:     router.HandleFunc,
	}
	s.started = svc
	return svc
}

func makeAnnotate(fn func(*funcOrigin)) func(interface{}) interface{} {
	if fn == nil {
		return nil
	}
	return func(f interface{}) interface{} {
		switch t := f.(type) {
		case *funcOrigin:
			fn(t)
			return t
		case *HandlerCollection:
			t.forEach(fn)
			return t
		default:
			fm := &funcOrigin{
				index:  -1,
				fn:     f,
				origin: "annotateSideEffects",
			}
			fn(fm)
			return fm
		}
	}
}
