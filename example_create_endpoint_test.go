package endpoint_test

import (
	"database/sql"
	"fmt"
	"net/http"

	"github.com/BlueOwlOpenSource/endpoint"
)

func WriteErrorResponse(inner func() endpoint.TerminalError, w http.ResponseWriter) {
	err := inner()
	if err != nil {
		w.Write([]byte(fmt.Sprintf("Could not open database: %s", err)))
		w.WriteHeader(500)
	}
}

func InjectDB(driver, uri string) func() (endpoint.TerminalError, *sql.DB) {
	return func() (endpoint.TerminalError, *sql.DB) {
		db, err := sql.Open(driver, uri)
		if err != nil {
			return err, nil
		}
		return nil, db
	}
}

func Example_CreateEndpoint() {
	mux := http.NewServeMux()
	mux.HandleFunc("/my/endpoint", endpoint.CreateEndpoint(
		WriteErrorResponse,
		InjectDB("postgres", "postgres://..."),
		func(r *http.Request, db *sql.DB, w http.ResponseWriter) error {
			// ....
			return nil
		}))
}
