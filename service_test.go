package endpoint_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/BlueOwlOpenSource/endpoint"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

func TestStaticInitializerWaitsForStart(t *testing.T) {
	t.Parallel()
	var initCount int
	var invokeCount int
	var tsiwfsFunc1 = func() int {
		t.Logf("endpoint init")
		initCount++
		return initCount
	}
	var tsiwfsFunc2 = func(w http.ResponseWriter, i int) {
		t.Logf("endpoint invoke")
		w.WriteHeader(204)
		invokeCount++
	}
	var svcInitCount int
	var svcInvokeCount int
	var svcTsiwfsFunc1 = func() string {
		t.Logf("service init")
		svcInitCount++
		return "foo"
	}
	var svcTsiwfsFunc2 = func(s string) {
		t.Logf("service invoke")
		svcInvokeCount++
	}
	multiStartups(
		t,
		"testWaitsForStart",
		endpoint.NewHandlerCollection("shc", svcTsiwfsFunc1, endpoint.AnnotateNotStatic(svcTsiwfsFunc2)),
		endpoint.NewHandlerCollectionWithEndpoint("hc", tsiwfsFunc1, tsiwfsFunc2),
		func(s string) {
			// reset
			t.Logf("reset for %s", s)
			initCount = 0
			invokeCount = 0
			svcInitCount = 10
			svcInvokeCount = 10
		},
		func(s string) {
			// after register
			assert.Equal(t, 0, initCount, s+" after register endpoint init count")
			assert.Equal(t, 0, invokeCount, s+" after register endpoint invoke count")
			assert.Equal(t, 10, svcInitCount, s+" after register service init count")
			assert.Equal(t, 10, svcInvokeCount, s+" after register service invoke count")
		},
		func(s string) {
			// after start
			assert.Equal(t, 1, initCount, s+" after start endpoint init count")
			assert.Equal(t, 0, invokeCount, s+" after start endpoint invoke count")
			assert.Equal(t, 11, svcInitCount, s+" after start service init count")
			assert.Equal(t, 10, svcInvokeCount, s+" after start service invoke count")
		},
		func(s string) {
			// after 1st call
			assert.Equal(t, 1, initCount, s+" 1st call start init count")
			assert.Equal(t, 1, invokeCount, s+" 1st call start invoke count")
			assert.Equal(t, 11, svcInitCount, s+" 1st call service init count")
			assert.Equal(t, 11, svcInvokeCount, s+" 1st call service invoke count")
		},
		func(s string) {
			// after 2nd call
			assert.Equal(t, 1, initCount, s+" 2nd call endpoint init count")
			assert.Equal(t, 2, invokeCount, s+" 2nd call endpoint invoke count")
			assert.Equal(t, 11, svcInitCount, s+" 2nd call service init count")
			assert.Equal(t, 12, svcInvokeCount, s+" 2nd call service invoke count")
		},
	)
}

func TestServiceInjectors(t *testing.T) {
	t.Parallel()
	var it1 intType1
	var se stringE
	var ecalled bool
	var icalled bool
	multiStartupsWithInject(
		t,
		"testServiceInjectors",
		nil,
		endpoint.NewHandlerCollectionWithEndpoint("tsi",
			endpoint.AutoInject(it1),
			endpoint.AutoInject(reflect.TypeOf(se)),
			func(e stringE) {
				assert.Equal(t, "e-val", string(e))
				ecalled = true
			},
			func(i intType1, w http.ResponseWriter) {
				assert.Equal(t, 37, int(i))
				w.WriteHeader(204)
				icalled = true
			},
		),
		func(s string) {
			// reset
			icalled = false
			ecalled = false
		},
		func(s string) {
			// after register
			assert.False(t, icalled)
			assert.False(t, ecalled)
		},
		func(s string) {
			// after start
			assert.False(t, icalled)
			assert.False(t, ecalled)
		},
		func(s string) {
			// after 1st call
			assert.True(t, icalled)
			assert.True(t, ecalled)
		},
		func(s string) {
			// after 2nd call
			assert.True(t, icalled)
			assert.True(t, ecalled)
		},
		[]interface{}{stringE("e-val"), intType1(37)},
	)
}

func TestFallibleInjectorFailing(t *testing.T) {
	t.Parallel()
	var initCount int
	var invokeCount int
	var errorsCount int
	multiStartups(
		t,
		"testFallibleInjectorFailing",
		nil,
		endpoint.NewHandlerCollectionWithEndpoint("hc",
			func(inner func() error, w http.ResponseWriter) {
				t.Logf("wraper (before)")
				err := inner()
				t.Logf("wraper (after, err=%v)", err)
				if err != nil {
					assert.Equal(t, "bailing out", err.Error())
					errorsCount++
					w.WriteHeader(204)
				}
			},
			func() (endpoint.TerminalError, int) {
				t.Logf("endpoint init")
				initCount++
				return fmt.Errorf("bailing out"), initCount
			},
			func(w http.ResponseWriter, i int) error {
				t.Logf("endpoint invoke")
				w.WriteHeader(204)
				invokeCount++
				return nil
			},
		),
		func(s string) {
			// reset
			t.Logf("reset for %s", s)
			initCount = 0
			invokeCount = 0
			errorsCount = 0
		},
		func(s string) {
			// after register
			assert.Equal(t, 0, initCount, s+" after register endpoint init count")
			assert.Equal(t, 0, invokeCount, s+" after register endpoint invoke count")
			assert.Equal(t, 0, errorsCount, s+" after register endpoint invoke count")
		},
		func(s string) {
			// after start
			assert.Equal(t, 0, initCount, s+" after start endpoint init count")
			assert.Equal(t, 0, invokeCount, s+" after start endpoint invoke count")
			assert.Equal(t, 0, errorsCount, s+" after register endpoint invoke count")
		},
		func(s string) {
			// after 1st call
			assert.Equal(t, 1, initCount, s+" 1st call start init count")
			assert.Equal(t, 0, invokeCount, s+" 1st call start invoke count")
			assert.Equal(t, 1, errorsCount, s+" after register endpoint invoke count")
		},
		func(s string) {
			// after 2nd call
			assert.Equal(t, 2, initCount, s+" 2nd call endpoint init count")
			assert.Equal(t, 0, invokeCount, s+" 2nd call endpoint invoke count")
			assert.Equal(t, 2, errorsCount, s+" after register endpoint invoke count")
		},
	)
}

func TestFallibleInjectorNotFailing(t *testing.T) {
	t.Parallel()
	var initCount int
	var invokeCount int
	var errorsCount int
	multiStartups(
		t,
		"testFallibleInjectorNotFailing",
		nil,
		endpoint.NewHandlerCollectionWithEndpoint("hc",
			func(inner func() error) {
				t.Logf("wraper (before)")
				err := inner()
				t.Logf("wraper (after, err=%v)", err)
				if err != nil {
					errorsCount++
				}
			},
			func() (endpoint.TerminalError, int) {
				t.Logf("endpoint init")
				initCount++
				return nil, 17
			},
			func(w http.ResponseWriter, i int) error {
				assert.Equal(t, 17, i)
				t.Logf("endpoint invoke")
				w.WriteHeader(204)
				invokeCount++
				return nil
			},
		),
		func(s string) {
			// reset
			t.Logf("reset for %s", s)
			initCount = 0
			invokeCount = 0
			errorsCount = 0
		},
		func(s string) {
			// after register
			assert.Equal(t, 0, initCount, s+" after register endpoint init count")
			assert.Equal(t, 0, invokeCount, s+" after register endpoint invoke count")
			assert.Equal(t, 0, errorsCount, s+" after register endpoint invoke count")
		},
		func(s string) {
			// after start
			assert.Equal(t, 0, initCount, s+" after start endpoint init count")
			assert.Equal(t, 0, invokeCount, s+" after start endpoint invoke count")
			assert.Equal(t, 0, errorsCount, s+" after register endpoint invoke count")
		},
		func(s string) {
			// after 1st call
			assert.Equal(t, 1, initCount, s+" 1st call start init count")
			assert.Equal(t, 1, invokeCount, s+" 1st call start invoke count")
			assert.Equal(t, 0, errorsCount, s+" after register endpoint invoke count")
		},
		func(s string) {
			// after 2nd call
			assert.Equal(t, 2, initCount, s+" 2nd call endpoint init count")
			assert.Equal(t, 2, invokeCount, s+" 2nd call endpoint invoke count")
			assert.Equal(t, 0, errorsCount, s+" after register endpoint invoke count")
		},
	)
}

func multiStartups(
	t *testing.T,
	name string,
	shc *endpoint.HandlerCollection,
	hc *endpoint.HandlerCollection,
	reset func(string),
	afterRegister func(string), // not called for CreateEnpoint
	afterStart func(string),
	afterCall1 func(string),
	afterCall2 func(string),
) {
	multiStartupsWithInject(t, name, shc, hc, reset, afterRegister, afterStart, afterCall1, afterCall2, nil)
	for {
		n := name + "-CreateEndpoint"
		reset(n)
		ept := "/" + n
		b := NewBinder()
		e := endpoint.CreateEndpoint(shc, hc)
		b.Bind(ept, e)
		// afterRegister(n)
		afterStart(n)
		b.Call(ept, "GET", "", nil)
		afterCall1(n)
		b.Call(ept, "GET", "", nil)
		afterCall2(n)
		break
	}
	for {
		n := name + "-RegisterService"
		reset(n)
		ept := "/" + n
		b := NewBinder()
		s := endpoint.RegisterService(n, b.Bind, shc)
		s.RegisterEndpoint(ept, hc)
		// afterRegister(n)
		afterStart(n)
		b.Call(ept, "GET", "", nil)
		afterCall1(n)
		b.Call(ept, "GET", "", nil)
		afterCall2(n)
		break
	}
	for {
		n := name + "-ServiceWithMux"
		reset(n)
		ept := "/" + n

		muxRouter := mux.NewRouter()

		s := endpoint.RegisterServiceWithMux(n, muxRouter, shc)
		s.RegisterEndpoint(ept, hc)
		// afterRegister(n)

		localServer := httptest.NewServer(muxRouter)
		defer localServer.Close()

		afterStart(n)

		t.Logf("GET %s%s\n", localServer.URL, ept)
		_, err := http.Get(localServer.URL + ept)
		assert.Nil(t, err)

		afterCall1(n)

		resp, err := http.Get(localServer.URL + ept)
		assert.Nil(t, err, name)
		assert.Equal(t, 204, resp.StatusCode, name)

		afterCall2(n)
		break
	}
}

func multiStartupsWithInject(
	t *testing.T,
	name string,
	shc *endpoint.HandlerCollection,
	hc *endpoint.HandlerCollection,
	reset func(string),
	afterRegister func(string), // not called for CreateEnpoint
	afterStart func(string),
	afterCall1 func(string),
	afterCall2 func(string),
	injections []interface{},
) {
	for {
		n := name + "-PreregisterService"
		reset(n)
		ept := "/" + n
		s := endpoint.PreRegisterService(n, shc)
		s.RegisterEndpoint(ept, hc)
		afterRegister(n)
		b := NewBinder()
		for _, i := range injections {
			s.Inject(i)
		}
		s.Start(b.Bind)
		afterStart(n)
		b.Call(ept, "GET", "", nil)
		afterCall1(n)
		b.Call(ept, "GET", "", nil)
		afterCall2(n)
		break
	}
	for {
		n := name + "-Prior-PreregisterService1"
		reset(n)
		ept := "/" + n
		s := endpoint.PreRegisterService(n, shc)
		b := NewBinder()
		for _, i := range injections {
			s.Inject(i)
		}
		s.Start(b.Bind)
		s.RegisterEndpoint(ept, hc)
		// afterRegister(n)
		afterStart(n)
		b.Call(ept, "GET", "", nil)
		afterCall1(n)
		b.Call(ept, "GET", "", nil)
		afterCall2(n)
		break
	}
	for {
		n := name + "-Prior-PreregisterService2"
		reset(n)
		ept := "/" + n
		s := endpoint.PreRegisterService(n, shc)
		b := NewBinder()
		for _, i := range injections {
			s.Inject(i)
		}
		sr := s.Start(b.Bind)
		sr.RegisterEndpoint(ept, hc)
		// afterRegister(n)
		afterStart(n)
		b.Call(ept, "GET", "", nil)
		afterCall1(n)
		b.Call(ept, "GET", "", nil)
		afterCall2(n)
		break
	}
	for {
		n := name + "-PreRegisterServiceWithMux"
		reset(n)
		ept := "/" + n
		s := endpoint.PreRegisterServiceWithMux(n, shc)
		s.RegisterEndpoint(ept, hc)
		afterRegister(n)

		for _, i := range injections {
			s.Inject(i)
		}
		muxRouter := mux.NewRouter()
		s.Start(muxRouter)

		localServer := httptest.NewServer(muxRouter)
		defer localServer.Close()

		afterStart(n)

		t.Logf("GET %s%s\n", localServer.URL, ept)
		_, err := http.Get(localServer.URL + ept)
		assert.Nil(t, err)

		afterCall1(n)

		resp, err := http.Get(localServer.URL + ept)
		assert.Nil(t, err, name)
		assert.Equal(t, 204, resp.StatusCode, name)

		afterCall2(n)
		break
	}
	for {
		n := name + "-PriorPreRegisterServiceWithMux1"
		reset(n)
		ept := "/" + n
		s := endpoint.PreRegisterServiceWithMux(n, shc)

		for _, i := range injections {
			s.Inject(i)
		}
		muxRouter := mux.NewRouter()
		s.Start(muxRouter)

		localServer := httptest.NewServer(muxRouter)
		defer localServer.Close()

		s.RegisterEndpoint(ept, hc)
		// afterRegister(n)

		afterStart(n)

		t.Logf("GET %s%s\n", localServer.URL, ept)
		_, err := http.Get(localServer.URL + ept)
		assert.Nil(t, err)

		afterCall1(n)

		resp, err := http.Get(localServer.URL + ept)
		assert.Nil(t, err, name)
		assert.Equal(t, 204, resp.StatusCode, name)

		afterCall2(n)
		break
	}
	for {
		n := name + "-PriorPreRegisterServiceWithMux2"
		reset(n)
		ept := "/" + n
		s := endpoint.PreRegisterServiceWithMux(n, shc)

		muxRouter := mux.NewRouter()
		for _, i := range injections {
			s.Inject(i)
		}
		sr := s.Start(muxRouter)

		localServer := httptest.NewServer(muxRouter)
		defer localServer.Close()

		sr.RegisterEndpoint(ept, hc)
		// afterRegister(n)

		afterStart(n)

		t.Logf("GET %s%s\n", localServer.URL, ept)
		_, err := http.Get(localServer.URL + ept)
		assert.Nil(t, err)

		afterCall1(n)

		resp, err := http.Get(localServer.URL + ept)
		assert.Nil(t, err, name)
		assert.Equal(t, 204, resp.StatusCode, name)

		afterCall2(n)
		break
	}
}

func TestMuxModifiers(t *testing.T) {
	t.Parallel()
	s := endpoint.PreRegisterServiceWithMux("TestCharacterize")

	s.RegisterEndpoint("/x", func(w http.ResponseWriter) {
		w.WriteHeader(204)
	}).Methods("GET")

	s.RegisterEndpoint("/x", func(w http.ResponseWriter) {
		w.WriteHeader(205)
	}).Methods("POST")

	muxRouter := mux.NewRouter()
	s.Start(muxRouter)

	localServer := httptest.NewServer(muxRouter)
	defer localServer.Close()

	resp, err := http.Get(localServer.URL + "/x")
	assert.Nil(t, err)
	assert.Equal(t, 204, resp.StatusCode)

	resp, err = http.Post(localServer.URL+"/x", "application/json", nil)
	assert.Nil(t, err)
	assert.Equal(t, 205, resp.StatusCode)
}
