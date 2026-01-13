// Copyright 2023 Canonical Ltd.
// Licensed under the LGPL license, see LICENSE file for details.

// Package mockhttp provides the ability to define mocked routes, specifying
// request validations to be performed on incoming requests to these routes,
// and mock responses to be returned for these requests.
package mockhttp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"

	qt "github.com/frankban/quicktest"
)

// Route represents a callable API endpoint.
type Route struct {
	// Method refers to the HTTP Method (http.MethodPost, http.MethodGet, etc).
	Method string
	// Endpoint refers to the route path specified either as an exact path or
	// as a regex prefixed with '=~'.
	// Example:
	//		`/stores`,
	//		`=~/stores/(w+)\z`   (matches '/stores/<store-id>')
	Endpoint string
}

// RouteResponder provides a way to define a mock http responder, wherein a
// request to a specific route can be validated as per some predefined
// expectation and mock responses can be configured to be returned when called.
type RouteResponder struct {
	// Route refers to a callable API endpoint.
	Route Route
	// req is used to temporarily store the incoming request to be validated
	// later.
	req *http.Request
	// ExpectedReqBody allows to specify the expected request body for requests
	// that call this Route.
	ExpectedReqBody any
	// ExpectedReqQueryParams allows to specify the expected request query
	// params for requests that call this Route.
	ExpectedReqQueryParams url.Values
	// ExpectedPathParams allows to specify the expected path parameters for
	// requests that call this Route, in the format name=value.
	ExpectedPathParams map[string]string
	// MockResponse allows to configure a mock response body to be returned.
	MockResponse any
	// MockResponseStatus allows to configure the response status to be used.
	// If not specified, defaults to http.StatusOK.
	MockResponseStatus int
}

// CreateMockServerWithResponders creates a mock HTTP server using the provided
// http.ServeMux. The server listens on the provided IP address and port, on the
// routes already present on the provided http.ServeMux and those provided as
// RouteResponder to this function.
func CreateMockServerWithResponders(mux *http.ServeMux, c *qt.C, ip string, port int, responders []*RouteResponder) *httptest.Server {
	for _, r := range responders {
		r.Register(mux)
	}

	ts := httptest.NewUnstartedServer(mux)

	// Replace the test server's IP and port with the expected ones.
	tl, err := net.Listen("tcp", fmt.Sprintf("%s:%d", ip, port))
	c.Assert(err, qt.IsNil)
	err = ts.Listener.Close()
	c.Assert(err, qt.IsNil)
	ts.Listener = tl

	return ts
}

// Register registers the Route with the provided http.ServeMux.
func (r *RouteResponder) Register(mux *http.ServeMux) {
	mux.HandleFunc(r.Route.Endpoint, func(w http.ResponseWriter, req *http.Request) {
		// Store the incoming request so that it can be validated later.
		// Note that the request body needs to be copied separately, as it can
		// only be read once.
		// See: https://pkg.go.dev/net/http#Request
		r.req = req.Clone(req.Context())
		body, err := io.ReadAll(req.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusInternalServerError)
			return
		}
		// Clone the request body, re-assigning it both to the stored request
		// and to the original request.
		// This allows later validations on r.req, while also ensuring that
		// further processing of req can still read the body.
		r.req.Body = io.NopCloser(bytes.NewReader(body))
		req.Body = io.NopCloser(bytes.NewReader(body))

		// If not specified, assume that the response status is http.StatusOK.
		if r.MockResponseStatus != 0 {
			if http.StatusText(r.MockResponseStatus) == "" {
				http.Error(w, fmt.Sprintf("invalid HTTP status code: %d", r.MockResponseStatus), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(r.MockResponseStatus)
		}

		if req.Method != r.Route.Method {
			http.Error(w, fmt.Sprintf("invalid HTTP method: got %s, want %s", req.Method, r.Route.Method), http.StatusMethodNotAllowed)
			return
		}

		if r.MockResponse != nil {
			w.Header().Set("Content-Type", "application/json")
			err = json.NewEncoder(w).Encode(r.MockResponse)
			if err != nil {
				http.Error(w, "failed to encode mock response", http.StatusInternalServerError)
				return
			}
		}
	})
}

// Validate runs validations for the route, ensuring that the received request
// matches the predefined expectations.
func (r *RouteResponder) Validate(c *qt.C) {
	if r.ExpectedReqBody != nil {
		body := make(map[string]any)
		err := json.NewDecoder(r.req.Body).Decode(&body)
		c.Assert(err, qt.IsNil)

		expReqBodyBytes, err := json.Marshal(r.ExpectedReqBody)
		c.Assert(err, qt.IsNil)
		expectedBody := make(map[string]any)
		err = json.Unmarshal(expReqBodyBytes, &expectedBody)
		c.Assert(err, qt.IsNil)

		c.Assert(body, qt.DeepEquals, expectedBody)
	}
	if r.ExpectedReqQueryParams != nil {
		c.Assert(r.req.URL.Query(), qt.ContentEquals, r.ExpectedReqQueryParams)
	}
	if r.ExpectedPathParams != nil {
		for pathParam, value := range r.ExpectedPathParams {
			got := r.req.PathValue(pathParam)
			c.Assert(got, qt.Equals, value, qt.Commentf("path parameter %s value mismatch", pathParam))
		}
	}
}
