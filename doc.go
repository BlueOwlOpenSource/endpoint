// Stuff

/*
Package endpoint is a general purpose web server framework that provides a
concise way to handle errors and inject dependencies into http endpoint
handlers.

Why?

Composite endpoints: endpoints are assembled from a collection handlers.  

Before/after actions: the middleware handler type wraps the rest of the
handler chain so that it can both inject items that are used downstream
and process return values.

Dependency injection: injectors, static injectors, and fallible injectors
can all be used to provide data and code to downstream handlers.  Downstream
handlers request what they need by including appropriate types in their
argument lists.  Injectors are invoked only if their outputs are consumed.

Code juxtiposition: when using pre-registered services, endpoint binding can
be registered next to the code that implements the endpoint even if the endpoints
are implemented in multiple files and/or packages.

Delayed initialization: initializers for pre-registered services are only executed
when the service is started and bound to an http server.  This allow code to define
such endpoints to depend on resources that may not be present unless the service
is started.

Reduced refactoring cost: handlers and endpoints declare their inputs and outputs
in the argument lists and return lists.  Handlers only need to know about their own
inputs and outputs.  The endpoint framework carries the data to where it is needed.
It does so with a minimum of copies and without recursive searches (see context.Context).
Type checking is done at service start time (or endpoint binding time when binding to
services that are already running).

Lower overhead indirect passing: when using context.Context to pass values indirectly,
the Go type system cannot be used to verify types at compile time or startup time.  Endpoint
verifies types at startup time allowing code that receives indirectly-passed data simpler.
As much as possible, work is done at initialization time rather than endpoint invocation time.

Basics

To use the endpoint package, create services first.  After that the
endpoints can be registered to the service and the service can be started.

Terminology

Service is a collection of endpoints that can be started together and may share
a handler collection.

Handler is a function that is used to help define an endpoint. 

Handler collection is a group of handlers.

Downstream handlers are handler that are to the right of the current hanlder
in the list of handlers.  They will be invoked after the current handler.

Upstream handlers are handlers that are to the left of the current handler
in the list of handlers.  They will have already been invoked by the time the
current handler is invoked.

Services

A service allows a group of related endpoints to be started together.
Each service may have a set of common handlers that are shared among
all the endpoints registered with that service.

Services come in four flavors: started or pre-registered; with Mux or
with without.

Pre-registered services are not initialized until they are Start()ed.  This
allows them to depend upon resources that may not may not be available without
causing a startup panic unless they're started without their required resrouces.
It also allows endpoints to be regisered in init() functions next to the
definition of the endpoint.

Handlers

A list of handlers will be invoked from left-to-right.  The first
handler in the list is invoked first and the last one (the endpoint)
is invoked last.  The handlers do not directly call each other --
rather the framework manages the invocations.  Data provided by one
handler can be used by any handler to its right and then as the
handlers return, the data returned can be used by any handler to its
left.  The data provided and required is identified by its type.
Since Go makes it easy to make aliases of types, it is easy to make
types distict.  When there is not an exact match of types, the framework
will use the closest (in distance) value that can convert to the
required type.

Each handler function is distinguished by its position in the
handler list and by its primary signature: its arguments
and return values.  In Go, types may be named or unnamed.  Unnamed function
types are part of primary signature.  Named function types are not part
of the primary signature.

These are the types that are recognized as valid handlers:
Static Injectors, Injectors, Endpoints, and Middleware.

Injectors are only invoked if their output is consumed or they have
no output.  Middleware handlers are (currently) always invoked.

*/
package endpoint
