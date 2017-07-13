package endpoint

import (
	"fmt"
	"net/http"
	"reflect"
)

// HandlerStaticInjectorType: handlers that match this type signature
// are invoked once at startup and never again.
//
// These should not return mutable objects because their
// return values are retained indefinitely and shared between
// simultaneous requests.
type HandlerStaticInjectorType func(TypeMoreInputsExcludingRequest) TypeMoreOutputsExcludingRequest

// HandlerInjectorType: handlers that match this type signature
// are invoked once per request (or more if re-invoked by middleware)
// Their return values are made available to all handlers to their right
// in the handler list.
//
// Injectors are called during the handling of a request.  They do not
// wrap the other calls.  The signature for injectors and static injectors
// looks the same!  The difference is that a static injector cannot take an
// argument of type http.ResponseWriter or *http.Request.  Once either of
// those types as been seen as a parameter in the handler list, then anything
// to the right of it
// that looks like an injector will be considered a regular injector rather
// than a static injector.
//
// Injectors are only invoked if their output values are consumed by
// another handler that will be executed.  Injectors that have no outputs
// are always invoked.
type HandlerInjectorType func(TypeMoreInputs) TypeMoreOutputs

// HandlerFallibleInjectorType: handlers taht match this type signature
// will be invoked once per request (or more if middleware
// re-invokes).  
//
// These are a special kind of injector: they can fail.
//
// If they fail, no
// additional injectors, or middleware will be invoked.  The endpoint handler
// will not be invoked.  The error return from the fallible injector must
// be caught by an earlier (to the left in the call chain) middleware handler.
//
// In all other respects these are the same as injectors that match
// the HandlerInjectorType type signature.
type HandlerFallibleInjectorType func(TypeMoreInputs) (TerminalError, TypeMoreOutputs)

// HandlerEndpointType: The function signature of an endpoint cannot be differentiated
// from the function signature of an injector.  An endpoint is simply the
// rightmost handler.  The last handler must be an endpoint.  The return
// values from an endpoint are made available to all preceeding handlers.
// All return values must be consumed by at least one handler.
type HandlerEndpointType func(TypeMoreInputs) TypeMoreReturnValues

// HandlerMiddlewareType: handlers that match this type signature wrap all
// downstream handlers.  They are invoked once per request (or more if middleware re-invokes).
//
// Middleware handlers take one function with an anonymous type.
// This function, we'll call it inner(), is called by the middleware
// function to invoke all the downstream handlers (that come after the middleware handler in
// the list of handlers)
//
// It can call inner() more than once if it wants to.  The parameters that the middlewar handler passes
// to inner() are made available to all handlers to the middleware's right.
// The values returned by inner() must be things that were returned by
// handlers after the middleware.  The return value from the middleware handler is made available
// to the handlers that came before the middleware handler.
type HandlerMiddlewareType func(TypeInnerType, TypeMoreInputs) TypeMoreReturnValues

// TypeInnerType is the signature of the the inner() function that is passed to
// the wrap() function for a middleware handler.  It's input values are
// made available to handlers after the middleware handler and its return
// values must be things returned from handlers after the middleware handler.
type TypeInnerType func(TypeMoreOutputs) TypeMoreReturnedValues

// TypeMoreInputsExcludingRequest is a dummy type used to document
// other types. In a list, this can be zero or more than one item.
// These additional parameters or return values must all have distinct
// types (no duplicates in the same set) and may not include un-typed func types.
// It is used to indicate a place where any number of parameters can go
type TypeMoreInputsExcludingRequest interface{}

// TypeMoreInputs is a dummy type used to document
// other types. In a list, this can be zero or more than one item.
// These additional parameters or return values must all have distinct
// types (no duplicates in the same set) and may not include un-typed func types.
// It is used to indicate a place where any number of parameters can go
type TypeMoreInputs interface{}

// TypeMoreOutputs is a dummy type used to document
// other types. In a list, this can be zero or more than one item.
// These additional parameters or return values must all have distinct
// types (no duplicates in the same set) and may not include un-typed func types.
// It is used to indicate a place where any number of parameters or return values can go
type TypeMoreOutputs interface{}

// TypeMoreOutputsExcludingRequest is a dummy type used to document
// other types. In a list, this can be zero or more than one item.
// These additional parameters or return values must all have distinct
// types (no duplicates in the same set) and may not include un-typed func types.
// It is used to indicate a place where any number of return values can go
type TypeMoreOutputsExcludingRequest interface{}

// TypeMoreReturnValues is a dummy type used to document
// other types. In a list, this can be zero or more than one item.
// These additional parameters or return values must all have distinct
// types (no duplicates in the same set) and may not include un-typed func types.
// It is used to indicate a place where any number of return values can go
type TypeMoreReturnValues interface{}

// TypeMoreReturnedValues is a dummy type used to document
// other types. In a list, this can be zero or more than one item.
// These additional parameters or return values must all have distinct
// types (no duplicates in the same set) and may not include un-typed func types.
// It is used to indicate a place where any number of return values can go
type TypeMoreReturnedValues interface{}

// TerminalError is a standard error interface.  For fallible injectors
// (matching the HandlerFallibleInjectorType type signature), TerminalError
// must be the first return value. 
// 
// A non-nil return value terminates the handler call chain.   All return
// values, including TerminalError must be consumed by an upstream handler.
type TerminalError interface {
	error
}

func init() {
	// TODO: make sure that all type are registered
	register(reflect.TypeOf((*TypeMoreInputs)(nil))).dummy().dummy().zeroOrMore().noAnonymous()
	register(reflect.TypeOf((*TypeMoreInputsExcludingRequest)(nil))).dummy().zeroOrMore().noAnonymous()
	register(reflect.TypeOf((*TypeMoreOutputs)(nil))).dummy().zeroOrMore().noAnonymous()
	register(reflect.TypeOf((*TypeMoreOutputsExcludingRequest)(nil))).dummy().zeroOrMore().noAnonymous()
	register(reflect.TypeOf((*TypeMoreReturnValues)(nil))).dummy().zeroOrMore().noAnonymous()
	register(reflect.TypeOf((*TypeMoreReturnedValues)(nil))).dummy().zeroOrMore().noAnonymous()
	register(reflect.TypeOf((*TypeInnerType)(nil))).dummy().anonymous().takes(outputParams).returns(returnedParams).noFlowType()
	register(reflect.TypeOf((*http.ResponseWriter)(nil))).dummy().notInStatic()
	register(reflect.TypeOf((**http.Request)(nil))).dummy().notInStatic()
	register(reflect.TypeOf((*noType)(nil))).dummy()
	register(reflect.TypeOf((*TerminalError)(nil))).dummy().returns(returnParams)
}

var responseWriterType = reflect.TypeOf((*http.ResponseWriter)(nil)).Elem()
var requestType = reflect.TypeOf((**http.Request)(nil)).Elem()
var errorType = reflect.TypeOf((*error)(nil)).Elem()
var terminalErrorType = reflect.TypeOf((*TerminalError)(nil)).Elem()

// The order of these variable assignments matter for the handlers
var middlewareFunc = register(reflect.TypeOf((*HandlerMiddlewareType)(nil))).
	notLast().group(middlewareGroup).pastStatic().takes(inputParams).returns(returnParams).
	FuncType

var endpointFunc = register(reflect.TypeOf((*HandlerEndpointType)(nil))).
	last().group(endpointGroup).pastStatic().takes(inputParams).returns(returnParams).
	FuncType

var fallibleInjectorFunc = register(reflect.TypeOf((*HandlerFallibleInjectorType)(nil))).
	notLast().group(middlewareGroup).pastStatic().takes(inputParams).returns(outputParams).
	FuncType

var staticInjectorFunc = register(reflect.TypeOf((*HandlerStaticInjectorType)(nil))).
	notLast().group(staticGroup).staticOnly().takes(inputParams).returns(outputParams).
	FuncType

var injectorFunc = register(reflect.TypeOf((*HandlerInjectorType)(nil))).
	notLast().group(middlewareGroup).pastStatic().takes(inputParams).returns(outputParams).
	FuncType

var allClasses = []typeCode{fallibleInjectorFunc, injectorFunc, middlewareFunc, endpointFunc, staticInjectorFunc}

const staticGroup = "static"
const middlewareGroup = "middleware"
const endpointGroup = "endpoint"

var allGroups = []string{staticGroup, middlewareGroup, endpointGroup}

const returnParams = "returns"    // going up
const outputParams = "outputs"    // going down
const inputParams = "inputs"      // recevied from above
const returnedParams = "returned" // received from below

// things returned by functions
var allReturns = []string{outputParams, returnedParams, returnParams}

// thing received by functions
var allTakes = []string{inputParams, outputParams}

type typeRegistration struct {
	Type          reflect.Type
	FuncType      typeCode
	Class         typeCode
	NoFlow        bool
	Last          bool // must be last in the list of functions
	NotLast       bool // may not be last in the list of functions
	CanSkip       bool // can skip over this if not present
	CanRepeat     bool // can repeat this
	Dummy         bool
	Group         string // which group of functions (used for service pregreg)
	Anonymous     bool   // only matches against anonymous functions
	NoAnonymous   bool   // does not match anonymous functions
	Takes         string
	Returns       string
	Preregistered bool
	NotInStatic   bool
	StaticOnly    bool
	PastStatic    bool
}

var primaryRegistrations = make([]*typeRegistration, 0, 20)
var registrations = make(map[typeCode]*typeRegistration)
var registrationsClosed = false

func register(pt reflect.Type) *typeRegistration {
	t := pt.Elem()
	r := getRegistration(t)
	if r.Preregistered {
		panic("duplicate registration")
	}
	r.Preregistered = true
	tc := GetTypeCode(t)
	registrations[tc] = r
	return r
}

func getRegistration(t reflect.Type) *typeRegistration {
	tc := GetTypeCode(t)
	r, preExisting := registrations[tc]
	if preExisting {
		return r
	}
	// TODO: register all the types in tests and then put this back
	// if registrationsClosed && t.PkgPath() == reflect.TypeOf((*HandlerEndpointType)(nil)).Elem().PkgPath() {
	//	panic(fmt.Sprintf("attempt to get registration too late on %s (%s)", t, t.PkgPath()))
	// }
	return &typeRegistration{
		Type:     t,
		FuncType: tc,
		Class:    tc,
	}
}

func (r *typeRegistration) last() *typeRegistration            { r.Last = true; return r }
func (r *typeRegistration) dummy() *typeRegistration           { r.Dummy = true; return r }
func (r *typeRegistration) notLast() *typeRegistration         { r.NotLast = true; return r }
func (r *typeRegistration) anonymous() *typeRegistration       { r.Anonymous = true; return r }
func (r *typeRegistration) noAnonymous() *typeRegistration     { r.NoAnonymous = true; return r }
func (r *typeRegistration) staticOnly() *typeRegistration      { r.StaticOnly = true; return r }
func (r *typeRegistration) notInStatic() *typeRegistration     { r.NotInStatic = true; return r }
func (r *typeRegistration) pastStatic() *typeRegistration      { r.PastStatic = true; return r }
func (r *typeRegistration) noFlowType() *typeRegistration      { r.NoFlow = true; return r }
func (r *typeRegistration) class(c typeCode) *typeRegistration { r.Class = c; return r }
func (r *typeRegistration) takes(i string) *typeRegistration   { r.Takes = i; return r }
func (r *typeRegistration) returns(o string) *typeRegistration { r.Returns = o; return r }
func (r *typeRegistration) zeroOrMore() *typeRegistration {
	r.CanSkip = true
	r.CanRepeat = true
	return r
}
func (r *typeRegistration) group(g string) *typeRegistration {
	primaryRegistrations = append(primaryRegistrations, r)
	r.Group = g
	return r
}

func typeCodesToMap(l []typeCode) map[typeCode]bool {
	m := make(map[typeCode]bool)
	for _, s := range l {
		m[s] = true
	}
	return m
}

func stringsToMap(l []string) map[string]bool {
	m := make(map[string]bool)
	for _, s := range l {
		m[s] = true
	}
	return m
}

var allNotInStatic []reflect.Type

func init() {
	for _, r := range registrations {
		if r.NotInStatic {
			allNotInStatic = append(allNotInStatic, r.Type)
		}
	}

	// This just double-checks the registrations
	groups := stringsToMap(allGroups)
	inputs := stringsToMap(allTakes)
	outputs := stringsToMap(allReturns)
	classes := typeCodesToMap(allClasses)
	for _, r := range registrations {
		if !r.Dummy && r.Group == "" {
			panic(fmt.Sprintf("registration needs dummy or group: %s", r.Type))
		}
		if r.Dummy && r.Group != "" {
			panic("registration cannot have both dummy and group")
		}
		if r.Group != "" && !groups[r.Group] {
			panic("group not in list of all groups")
		}
		if r.Takes != "" && !inputs[r.Takes] {
			panic("input not in list of all inputs")
		}
		if r.Returns != "" && !outputs[r.Returns] {
			panic("input not in list of all outputs")
		}
		if !r.Dummy && !classes[r.Class] {
			panic(fmt.Sprintf("input not in list of all classes: %s", r.Class.Type()))
		}
	}
	if len(registrations) == 0 {
		panic("there should be registrations")
	}
	registrationsClosed = true
}
