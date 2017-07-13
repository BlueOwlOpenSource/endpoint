# endpoint

[GoDoc](https://godoc.org/github.com/BlueOwlOpenSource/endpoint)

This package attempts to solve several issues with http endpoint handlers:

 * Chaining actions to create composite handlers 
 * Wrapping endpoint handlers with before/after actions
 * Dependency injection for endpoint handlers
 * Binding handlers to their endpoints next to the code that defines the endpoints
 * Delaying initialization code execution until services are started allowing services that are not used to remain uninitialized
 * Reducing the cost of refactoring endpoing handlers passing data directly from producers to consumers without needing intermediaries to care about what is being passed
 * Avoid the overhead of verifying that requested items are present when passed indrectly (a common problem when using context.Context to pass data indirectly)

It does this by defining endpoints as a sequence of handler functions.  Handler functions
come in different flavors.  Data is passed from one handler to the next using reflection to 
match the output types with input types requested later in the handler chain.  To the extent
possible, the handling is pre-computed so there is very little reflection happening when the
endpoint is actually invoked.

Endpoints are registered to services before or after the service is started.

When services are pre-registered and started later, it is possible to bind endpoints
to them in init() functions and thus colocate the endpoint binding with the endpoint
definition and spread the endpoint definition across multiple files and/or packages.

Services come in two flavors: one flavor binds endpoints with http.HandlerFunc and the
other binds using mux.Router.  

Example useses include:

 * Turning output structs into json
 * Common error handling
 * Common logging
 * Common initialization
 * Injection of resources and dependencies
