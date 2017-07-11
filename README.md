# endpoint

[GoDoc](https://godoc.org/github.com/BlueOwlOpenSource/endpoint)

This package provides way to organize injectors and wrappers when creating http handlers (endpoints).
When using the pre-register variants of creating services, it allows individual endpoints to
self-register.

Services can be pre-registered and started later or they can be registered
and stated immediately.  Services can be started with a binder that matches
http.HandlerFunc, or they can be started with a *mux.Router.  Combined with
the pre-register vs. start immedidately, services come in four flavors:
ServiceRegistration, Service, ServiceRegistrationWithMux, and ServiceWithMux.

Endpoint provides a clean way to wrap endpoint implementations and factor out common
code from endpoints.  Example useses include:

 * Turning output structs into json
 * Common error handling
 * Common logging
 * Common initialization
 * Injection of resources and dependencies

