package endpoint_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	"github.com/BlueOwlOpenSource/endpoint"
	"github.com/gorilla/mux"
)

type exampleType string

// exampleStaticInjector will not be called until the service.Start()
// call in Example_PreRegisterServiceWithMux.  It will be called only
// once per endpoint registered.
func exampleStaticInjector() exampleType {
	return "example static value"
}

type returnValue interface{}

func jsonifyResult(inner func() returnValue, w http.ResponseWriter) {
	v := inner()
	w.Header().Set("Content-Type", "application/json")
	encoded, _ := json.Marshal(v)
	w.Write(encoded)
	w.WriteHeader(200)
}

var service = endpoint.PreRegisterServiceWithMux("example-service",
	exampleStaticInjector,
	jsonifyResult)

func init() {
	service.RegisterEndpoint("/example",
		func(sv exampleType) returnValue {
			return map[string]string{
				"static value": string(sv),
			}
		})
}

// Example_PreRegisterServiceWithMux demonstrates use of service pre-registration.
func Example() {
	muxRouter := mux.NewRouter()
	service.Start(muxRouter)
	localServer := httptest.NewServer(muxRouter)
	defer localServer.Close()
	r, err := http.Get(localServer.URL + "/example")
	if err != nil {
		fmt.Println("get error", err)
		return
	}
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fmt.Println("read error", err)
		return
	}
	var res map[string]string
	err = json.Unmarshal(buf, &res)
	if err != nil {
		fmt.Println("unmarshal error", err)
		return
	}
	fmt.Println("Static:", res["static value"])
	// Output: Static: example static value
}
