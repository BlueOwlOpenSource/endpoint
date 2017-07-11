package endpoint_test

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/BlueOwlOpenSource/endpoint"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

type RequestBody []byte

func LogError(inner func() error) {
	// actaully, ignore error
	inner()
}

func SaveRequest(inner func(RequestBody) error, r *http.Request) error {
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}
	r.Body = ioutil.NopCloser(bytes.NewBuffer(buf))
	return inner(buf)
}

func TestSaveRequest(t *testing.T) {
	t.Parallel()
	calledPost := false
	calledGet := false
	s := endpoint.PreRegisterServiceWithMux("TestSaveRequest", LogError, SaveRequest)

	s.RegisterEndpoint("/ept", func(body RequestBody, w http.ResponseWriter) error {
		w.WriteHeader(204)
		assert.Equal(t, "some stuff", string(body))
		calledPost = true
		return nil
	}).Methods("POST")

	s.RegisterEndpoint("/ept", func(w http.ResponseWriter) error {
		w.WriteHeader(204)
		calledGet = true
		return nil
	}).Methods("GET")

	muxRouter := mux.NewRouter()
	assert.False(t, calledPost)
	assert.False(t, calledGet)
	s.Start(muxRouter)
	assert.False(t, calledPost)
	assert.False(t, calledGet)

	localServer := httptest.NewServer(muxRouter)
	defer localServer.Close()

	_, err := http.Post(localServer.URL+"/ept", "text/plain", ioutil.NopCloser(bytes.NewBuffer([]byte("some stuff"))))
	assert.Nil(t, err)
	assert.True(t, calledPost)
	assert.False(t, calledGet)

	_, err = http.Get(localServer.URL + "/ept")
	assert.Nil(t, err)
	assert.True(t, calledGet)
}
