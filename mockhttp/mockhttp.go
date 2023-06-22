// Package mockhttp provides the ability to define mocked routes, specifying
// request validations to be performed on incoming requests to these routes,
// and mock responses to be returned for these requests.
package mockhttp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	qt "github.com/frankban/quicktest"
	"github.com/jarcoal/httpmock"
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
	// requests that call this Route. They should be specified in the order
	// that they are expected to be found in the path.
	ExpectedPathParams []string
	// MockResponse allows to configure a mock response body to be returned.
	MockResponse any
	// MockResponseStatus allows to configure the response status to be used.
	// If not specified, defaults to http.StatusOK.
	MockResponseStatus int
}

// isValidHTTPStatusCode checks whether the input code refers to a valid HTTP
// Status code.
func isValidHTTPStatusCode(code int) bool {
	return http.StatusText(code) != ""
}

// Generate returns a httpmock.Responder function for the Route. This returned
// function is used by httpmock to Generate a response whenever a http request
// is made to the configured Route.
func (r *RouteResponder) Generate() httpmock.Responder {
	return func(req *http.Request) (*http.Response, error) {
		// Store the incoming request so that it can be validated later.
		r.req = req

		status := http.StatusOK
		if r.MockResponseStatus != 0 {
			if !isValidHTTPStatusCode(r.MockResponseStatus) {
				panic(fmt.Sprintf("Invalid HTTP status code: %d", r.MockResponseStatus))
			}
			status = r.MockResponseStatus
		}
		resp, err := httpmock.NewJsonResponse(status, r.MockResponse)
		if err != nil {
			return httpmock.NewStringResponse(http.StatusInternalServerError, "failed to convert mockResponse to json"), nil
		}
		return resp, nil
	}
}

// Finish runs validations for the route, ensuring that the received request
// matches the predefined expectations.
func (r *RouteResponder) Finish(c *qt.C) {
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
		for i, expected := range r.ExpectedPathParams {
			got := httpmock.MustGetSubmatch(r.req, i+1)
			c.Assert(got, qt.Equals, expected, qt.Commentf("path parameter mismatch"))
		}
	}
}
