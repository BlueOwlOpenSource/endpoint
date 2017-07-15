package endpoint

import (
	"fmt"
	"net/http"
	"reflect"
)

type handlerStaticInjectorType func(typeMoreInputsExcludingRequest) typeMoreOutputsExcludingRequest

type handlerInjectorType func(typeMoreInputs) typeMoreOutputs

type handlerFallibleInjectorType func(typeMoreInputs) (TerminalError, typeMoreOutputs)

type handlerEndpointType func(typeMoreInputs) typeMoreReturnValues

type handlerMiddlewareType func(typeInnerType, typeMoreInputs) typeMoreReturnValues

type typeInnerType func(typeMoreOutputs) typeMoreReturnedValues

type typeMoreInputsExcludingRequest interface{}

type typeMoreInputs interface{}

type typeMoreOutputs interface{}

type typeMoreOutputsExcludingRequest interface{}

type typeMoreReturnValues interface{}

type typeMoreReturnedValues interface{}

// TerminalError is a standard error interface.  For fallible injectors
// (matching the handlerFallibleInjectorType type signature), TerminalError
// must be the first return value.
//
// A non-nil return value terminates the handler call chain.   All return
// values, including TerminalError must be consumed by an upstream handler.
type TerminalError interface {
	error
}

func init() {
	// TODO: make sure that all type are registered
	register(reflect.TypeOf((*typeMoreInputs)(nil))).dummy().dummy().zeroOrMore().noAnonymous()
	register(reflect.TypeOf((*typeMoreInputsExcludingRequest)(nil))).dummy().zeroOrMore().noAnonymous()
	register(reflect.TypeOf((*typeMoreOutputs)(nil))).dummy().zeroOrMore().noAnonymous()
	register(reflect.TypeOf((*typeMoreOutputsExcludingRequest)(nil))).dummy().zeroOrMore().noAnonymous()
	register(reflect.TypeOf((*typeMoreReturnValues)(nil))).dummy().zeroOrMore().noAnonymous()
	register(reflect.TypeOf((*typeMoreReturnedValues)(nil))).dummy().zeroOrMore().noAnonymous()
	register(reflect.TypeOf((*typeInnerType)(nil))).dummy().anonymous().takes(outputParams).returns(returnedParams).noFlowType()
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
var middlewareFunc = register(reflect.TypeOf((*handlerMiddlewareType)(nil))).
	notLast().group(middlewareGroup).pastStatic().takes(inputParams).returns(returnParams).
	FuncType

var endpointFunc = register(reflect.TypeOf((*handlerEndpointType)(nil))).
	last().group(endpointGroup).pastStatic().takes(inputParams).returns(returnParams).
	FuncType

var fallibleInjectorFunc = register(reflect.TypeOf((*handlerFallibleInjectorType)(nil))).
	notLast().group(middlewareGroup).pastStatic().takes(inputParams).returns(outputParams).
	FuncType

var staticInjectorFunc = register(reflect.TypeOf((*handlerStaticInjectorType)(nil))).
	notLast().group(staticGroup).staticOnly().takes(inputParams).returns(outputParams).
	FuncType

var injectorFunc = register(reflect.TypeOf((*handlerInjectorType)(nil))).
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
	// if registrationsClosed && t.PkgPath() == reflect.TypeOf((*handlerEndpointType)(nil)).Elem().PkgPath() {
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
