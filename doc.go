// Stuff

/*
Package endpoint is a general purpose web server framework that provides a
concise way to handle errors and inject dependencies into http endpoint
handlers.

Endpoints are assembled from a collection handlers.  As much as possible,
work is done at initialization time rather than endpoint invocation time.

To use the endpoint package, create services first.  After that the
endpoints can be registered to the service and the service can be started.

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
