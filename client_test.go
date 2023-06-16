// Strategy:
// Due to the design of the underlying openfga client, there is no direct way
// to test the integration of this wrapper library with the underlying client.
//
// However, we can test this integration indirectly by using http mocks.
// We know that our ofga wrapper communicates with the openfga client, which in
// turn uses a http client to talk to the actual openfga instance.
//
// 	ofga wrapper <---> openfga client <---> http client <---> openfga instance
//
// If we mock the http client, we can indirectly test the integration between
// our wrapper and the openfga client.
//
//  	ofga wrapper <---> openfga client <---> http mock
//
// This can be done by:
//	- calling specific methods on the wrapper and ensuring that the mock http
//		client receives the expected request bodies.
//  - having the mock http client respond with mock responses and ensuring that
//		the wrapper receives the expected responses.

package ofga_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/jarcoal/httpmock"
	openfga "github.com/openfga/go-sdk"

	"github.com/canonical/ofga"
)

var FGAParams = ofga.OpenFGAParams{
	Scheme:      "http",
	Host:        "localhost",
	Port:        "8080",
	Token:       "InsecureTokenDoNotUse",
	StoreID:     "TestStoreID",
	AuthModelID: "TestAuthModelID",
}

// Route represents a callable API endpoint.
type Route struct {
	// The http method - http.MethodPost, http.MethodGet, etc
	method string
	// The endpoint specified as an exact path, or a regex prefixed with '=~'
	// example:
	//	`/stores`,
	//	`=~/stores/(w+)\z`   (matches '/stores/<store-id>')
	endpoint string
}

// RouteResponder provides a way to define a mock http responder, wherein the
// request body can be validated as per expectation and mock responses can be
// returned.
type RouteResponder struct {
	route Route
	// body is for internal usage. It is used to temporarily store the incoming
	// request body to be validated later and should not be set manually.
	body io.ReadCloser
	// expectedRequest allows to specify the expected request body for requests
	// that call this route.
	expectedRequest any
	// TODO Path params and expected path params
	// TODO Query params and expected query params
	// TODO Check that method params are validated in the expectedRequest
	// mockResponse allows to configure the exact response body to be returned.
	mockResponse any
	// mockResponseStatus allows to configure the response status to be used.
	// If not specified, defaults to http.StatusOK.
	mockResponseStatus int
}

// generate returns a httpmock.Responder function for the Route. This returned
// function is used by httpmock to generate a response whenever a http request
// is made to this route.
func (r *RouteResponder) generate() httpmock.Responder {
	return func(req *http.Request) (*http.Response, error) {
		r.body = req.Body

		status := http.StatusOK
		if r.mockResponseStatus != 0 {
			status = r.mockResponseStatus
		}
		resp, err := httpmock.NewJsonResponse(status, r.mockResponse)
		if err != nil {
			return httpmock.NewStringResponse(http.StatusInternalServerError, "failed to convert mockResponse to json"), nil
		}
		return resp, nil
	}
}

// finish runs validations for the route, ensuring that the received request
// body matches the expected request body.
func (r *RouteResponder) finish(c *qt.C) {
	if r.expectedRequest != nil {
		body := make(map[string]any)
		err := json.NewDecoder(r.body).Decode(&body)
		c.Assert(err, qt.IsNil, qt.Commentf("received request body is in incorrect format: %s", err))

		expectedBody := make(map[string]any)
		marshal, err := json.Marshal(r.expectedRequest)
		c.Assert(err, qt.IsNil, qt.Commentf("expectedReqBody is in incorrect format: %s", err))
		err = json.Unmarshal(marshal, &expectedBody)
		c.Assert(err, qt.IsNil, qt.Commentf("expectedReqBody is in incorrect format: %s", err))

		c.Assert(body, qt.DeepEquals, expectedBody)
	}
}

var (
	Check          = Route{method: http.MethodPost, endpoint: `=~/stores/(\w+)/check\z`}
	CreateStore    = Route{method: http.MethodPost, endpoint: "/stores"}
	Expand         = Route{method: http.MethodPost, endpoint: `=~/stores/(\w+)/expand\z`}
	GetStore       = Route{method: http.MethodGet, endpoint: `=~/stores/(\w+)\z`}
	ListObjects    = Route{method: http.MethodPost, endpoint: `=~/stores/(\w+)/list-objects\z`}
	ListStore      = Route{method: http.MethodGet, endpoint: "/stores"}
	Read           = Route{method: http.MethodGet, endpoint: `=~/stores/(\w+)/read\z`}
	ReadAuthModel  = Route{method: http.MethodGet, endpoint: `=~/stores/(\w+)/authorization-models/(\w+)\z`}
	ReadAuthModels = Route{method: http.MethodGet, endpoint: `=~/stores/(\w+)/authorization-models\z`}
	ReadChanges    = Route{method: http.MethodGet, endpoint: `=~/stores/(\w+)/changes\z`}
	Write          = Route{method: http.MethodPost, endpoint: `=~/stores/(\w+)/write\z`}
	WriteAuthModel = Route{method: http.MethodPost, endpoint: `=~/stores/(\w+)/authorization-models\z`}
)

// getTestClient creates and returns an ofga.Client. It takes care of setting
// up all the mock http routes required by the client during the initialization
// process.
func getTestClient(c *qt.C) *ofga.Client {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	clientCreationRoutes := []*RouteResponder{{
		route: ListStore,
	}, {
		route: GetStore,
		mockResponse: openfga.GetStoreResponse{
			Id:   openfga.PtrString(FGAParams.StoreID),
			Name: openfga.PtrString("Test Store"),
		},
	}, {
		route: ReadAuthModel,
		mockResponse: openfga.ReadAuthorizationModelResponse{AuthorizationModel: &openfga.AuthorizationModel{
			Id:            openfga.PtrString(FGAParams.AuthModelID),
			SchemaVersion: "1.1",
		}},
	}}

	for _, cr := range clientCreationRoutes {
		httpmock.RegisterResponder(cr.route.method, cr.route.endpoint, cr.generate())
	}

	// Create a client
	newClient, err := ofga.NewClient(context.Background(), FGAParams)
	c.Assert(err, qt.IsNil)
	c.Assert(newClient.AuthModelId, qt.Equals, FGAParams.AuthModelID)

	for _, cr := range clientCreationRoutes {
		cr.finish(c)
	}
	return newClient
}

func TestNewClient(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	tests := []struct {
		about               string
		params              ofga.OpenFGAParams
		mockRoutes          []*RouteResponder
		expectedErr         string
		expectedAuthModelID string
	}{{
		about: "client creation fails when Host param is missing",
		params: ofga.OpenFGAParams{
			Scheme:      "http",
			Host:        "",
			Port:        "8080",
			Token:       "InsecureTokenDoNotUse",
			StoreID:     "TestStoreID",
			AuthModelID: "TestAuthModelID",
		},
		expectedErr: "OpenFGA configuration: missing host",
	}, {
		about: "client creation fails when Port param is missing",
		params: ofga.OpenFGAParams{
			Scheme:      "http",
			Host:        "localhost",
			Port:        "",
			Token:       "InsecureTokenDoNotUse",
			StoreID:     "TestStoreID",
			AuthModelID: "TestAuthModelID",
		},
		expectedErr: "OpenFGA configuration: missing port",
	}, {
		about: "client creation fails when AuthModelID is specified without a StoreID",
		params: ofga.OpenFGAParams{
			Scheme:      "http",
			Host:        "localhost",
			Port:        "8080",
			Token:       "InsecureTokenDoNotUse",
			StoreID:     "",
			AuthModelID: "TestAuthModelID",
		},
		expectedErr: "OpenFGA configuration: AuthModelID specified without a StoreID",
	}, {
		about: "client creation fails when any other configuration issue occurs",
		params: ofga.OpenFGAParams{
			Scheme:      "invalidScheme",
			Host:        "localhost",
			Port:        "8080",
			Token:       "InsecureTokenDoNotUse",
			StoreID:     "TestStoreID",
			AuthModelID: "TestAuthModelID",
		},
		expectedErr: "OpenFGA configuration: .*",
	}, {
		about:  "client creation fails when we are unable to list stores from openFGA",
		params: FGAParams,
		mockRoutes: []*RouteResponder{{
			route:              ListStore,
			mockResponseStatus: http.StatusInternalServerError,
		}},
		expectedErr: "cannot list stores.*",
	}, {
		about:  "client creation fails when StoreID is specified but the Get Store request returns an error",
		params: FGAParams,
		mockRoutes: []*RouteResponder{{
			route: ListStore,
		}, {
			route:              GetStore,
			mockResponseStatus: http.StatusInternalServerError,
		}},
		expectedErr: "cannot retrieve store.*",
	}, {
		about:  "client creation fails when AuthModelID is specified but the Read Authorization Model request returns an error",
		params: FGAParams,
		mockRoutes: []*RouteResponder{{
			route: ListStore,
		}, {
			route:        GetStore,
			mockResponse: openfga.GetStoreResponse{Name: openfga.PtrString("Test Store")},
		}, {
			route:              ReadAuthModel,
			mockResponseStatus: http.StatusInternalServerError,
		}},
		expectedErr: "cannot retrieve authModel.*",
	}, {
		about:  "client created successfully",
		params: FGAParams,
		mockRoutes: []*RouteResponder{{
			route: ListStore,
		}, {
			route:        GetStore,
			mockResponse: openfga.GetStoreResponse{Name: openfga.PtrString("Test Store")},
		}, {
			route: ReadAuthModel,
			mockResponse: openfga.ReadAuthorizationModelResponse{
				AuthorizationModel: &openfga.AuthorizationModel{
					Id: openfga.PtrString(FGAParams.AuthModelID),
				},
			},
		}},
		expectedAuthModelID: FGAParams.AuthModelID,
	}}

	for _, test := range tests {
		test := test
		c.Run(test.about, func(c *qt.C) {
			// Set up and configure the http mocks
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()
			for _, mr := range test.mockRoutes {
				httpmock.RegisterResponder(mr.route.method, mr.route.endpoint, mr.generate())
			}

			// Execute the test
			client, err := ofga.NewClient(ctx, test.params)

			if test.expectedErr != "" {
				c.Assert(err, qt.ErrorMatches, test.expectedErr)
				c.Assert(client, qt.IsNil)
			} else {
				c.Assert(err, qt.IsNil)
				c.Assert(client.AuthModelId, qt.Equals, test.expectedAuthModelID)
			}

			// Validate that the mock routes were called as expected.
			for _, mr := range test.mockRoutes {
				mr.finish(c)
			}
		})
	}
}

func TestClient_AddRelation(t *testing.T) {
	c := qt.New(t)

	ctx := context.Background()
	client := getTestClient(c)

	tests := []struct {
		about       string
		tuples      []ofga.Tuple
		mockRoutes  []*RouteResponder
		expectedErr string
	}{{
		about: "error returned by the client is returned to the caller",
		tuples: []ofga.Tuple{
			{
				Object:   &entityTestUser,
				Relation: relationEditor,
				Target:   &entityTestContract,
			},
		},
		mockRoutes: []*RouteResponder{{
			route:              Write,
			mockResponseStatus: http.StatusBadRequest,
		}},
		expectedErr: "cannot add relation .*",
	}, {
		about: "relation added successfully",
		tuples: []ofga.Tuple{
			{
				Object:   &entityTestUser,
				Relation: relationEditor,
				Target:   &entityTestContract,
			},
		},
		mockRoutes: []*RouteResponder{{
			route: Write,
			expectedRequest: openfga.WriteRequest{
				Writes: openfga.NewTupleKeys([]openfga.TupleKey{{
					User:     openfga.PtrString(entityTestUser.String()),
					Relation: openfga.PtrString(relationEditor.String()),
					Object:   openfga.PtrString(entityTestContract.String()),
				}}),
				AuthorizationModelId: openfga.PtrString(FGAParams.AuthModelID),
			},
		}},
	}}

	for _, test := range tests {
		test := test
		c.Run(test.about, func(c *qt.C) {
			// Set up and configure mock http responders.
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()
			for _, mr := range test.mockRoutes {
				httpmock.RegisterResponder(mr.route.method, mr.route.endpoint, mr.generate())
			}

			// Execute the test
			err := client.AddRelation(ctx, test.tuples...)

			if test.expectedErr != "" {
				c.Assert(err, qt.ErrorMatches, test.expectedErr)
			} else {
				c.Assert(err, qt.IsNil)
			}

			// Validate that the mock routes were called as expected.
			for _, mr := range test.mockRoutes {
				mr.finish(c)
			}
		})
	}
}

func TestClient_CheckRelation(t *testing.T) {
	c := qt.New(t)

	ctx := context.Background()
	client := getTestClient(c)

	tests := []struct {
		about           string
		tuple           ofga.Tuple
		mockRoutes      []*RouteResponder
		expectedAllowed bool
		expectedErr     string
	}{{
		about: "error returned by the client is returned to the caller",
		tuple: ofga.Tuple{
			Object:   &entityTestUser,
			Relation: relationEditor,
			Target:   &entityTestContract,
		},
		mockRoutes: []*RouteResponder{{
			route:              Check,
			mockResponseStatus: http.StatusInternalServerError,
		}},
		expectedErr: "cannot check relation .*",
	}, {
		about: "relation checked successfully",
		tuple: ofga.Tuple{
			Object:   &entityTestUser,
			Relation: relationEditor,
			Target:   &entityTestContract,
		},
		mockRoutes: []*RouteResponder{{
			route: Check,
			expectedRequest: openfga.CheckRequest{
				TupleKey: openfga.TupleKey{
					User:     openfga.PtrString(entityTestUser.String()),
					Relation: openfga.PtrString(relationEditor.String()),
					Object:   openfga.PtrString(entityTestContract.String()),
				},
				AuthorizationModelId: openfga.PtrString(FGAParams.AuthModelID),
			},
			mockResponse: openfga.CheckResponse{
				Allowed: openfga.PtrBool(true),
			},
		}},
		expectedAllowed: true,
	}}

	for _, test := range tests {
		test := test
		c.Run(test.about, func(c *qt.C) {
			// Set up and configure mock http responders.
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()
			for _, mr := range test.mockRoutes {
				httpmock.RegisterResponder(mr.route.method, mr.route.endpoint, mr.generate())
			}

			// Execute the test
			allowed, err := client.CheckRelation(ctx, test.tuple)

			if test.expectedErr != "" {
				c.Assert(err, qt.ErrorMatches, test.expectedErr)
			} else {
				c.Assert(err, qt.IsNil)
				c.Assert(allowed, qt.Equals, test.expectedAllowed)
			}

			// Validate that the mock routes were called as expected.
			for _, mr := range test.mockRoutes {
				mr.finish(c)
			}
		})
	}
}

func TestClient_RemoveRelation(t *testing.T) {
	c := qt.New(t)

	ctx := context.Background()
	client := getTestClient(c)

	tests := []struct {
		about       string
		tuples      []ofga.Tuple
		mockRoutes  []*RouteResponder
		expectedErr string
	}{{
		about: "error returned by the client is returned to the caller",
		tuples: []ofga.Tuple{{
			Object:   &entityTestUser,
			Relation: relationEditor,
			Target:   &entityTestContract,
		}},
		mockRoutes: []*RouteResponder{{
			route:              Write,
			mockResponseStatus: http.StatusInternalServerError,
		}},
		expectedErr: "cannot remove relation .*",
	}, {
		about: "relation removed successfully",
		tuples: []ofga.Tuple{{
			Object:   &entityTestUser,
			Relation: relationEditor,
			Target:   &entityTestContract,
		}},
		mockRoutes: []*RouteResponder{{
			route: Write,
			expectedRequest: openfga.WriteRequest{
				Deletes: openfga.NewTupleKeys([]openfga.TupleKey{{
					User:     openfga.PtrString(entityTestUser.String()),
					Relation: openfga.PtrString(relationEditor.String()),
					Object:   openfga.PtrString(entityTestContract.String()),
				}}),
				AuthorizationModelId: openfga.PtrString(FGAParams.AuthModelID),
			},
		}},
	}}

	for _, test := range tests {
		test := test
		c.Run(test.about, func(c *qt.C) {
			// Set up and configure mock http responders.
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()
			for _, mr := range test.mockRoutes {
				httpmock.RegisterResponder(mr.route.method, mr.route.endpoint, mr.generate())
			}

			// Execute the test
			err := client.RemoveRelation(ctx, test.tuples...)

			if test.expectedErr != "" {
				c.Assert(err, qt.ErrorMatches, test.expectedErr)
			} else {
				c.Assert(err, qt.IsNil)
			}

			// Validate that the mock routes were called as expected.
			for _, mr := range test.mockRoutes {
				mr.finish(c)
			}
		})
	}
}

func TestClient_CreateStore(t *testing.T) {
	c := qt.New(t)

	ctx := context.Background()
	client := getTestClient(c)

	tests := []struct {
		about           string
		storeName       string
		mockRoutes      []*RouteResponder
		expectedStoreID string
		expectedErr     string
	}{{
		about:     "error returned by the client is returned to the caller",
		storeName: "TestStore",
		mockRoutes: []*RouteResponder{{
			route:              CreateStore,
			expectedRequest:    openfga.CreateStoreRequest{Name: "TestStore"},
			mockResponseStatus: http.StatusInternalServerError,
		}},
		expectedErr: "cannot create store.*",
	}, {
		about:     "store is created successfully",
		storeName: "TestStore",
		mockRoutes: []*RouteResponder{{
			route:           CreateStore,
			expectedRequest: openfga.CreateStoreRequest{Name: "TestStore"},
			mockResponse:    openfga.CreateStoreResponse{Id: openfga.PtrString("12345")},
		}},
		expectedStoreID: "12345",
		expectedErr:     "",
	}}

	for _, test := range tests {
		test := test
		c.Run(test.about, func(c *qt.C) {
			// Set up and configure mock http responders.
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()
			for _, mr := range test.mockRoutes {
				httpmock.RegisterResponder(mr.route.method, mr.route.endpoint, mr.generate())
			}

			// Execute the test
			storeID, err := client.CreateStore(ctx, test.storeName)

			if test.expectedErr != "" {
				c.Assert(err, qt.ErrorMatches, test.expectedErr)
				c.Assert(storeID, qt.Equals, "")
			} else {
				c.Assert(err, qt.IsNil)
				c.Assert(storeID, qt.Equals, test.expectedStoreID)
			}

			// Validate that the mock routes were called as expected.
			for _, mr := range test.mockRoutes {
				mr.finish(c)
			}
		})
	}
}

func TestClient_ListStores(t *testing.T) {
	c := qt.New(t)

	ctx := context.Background()
	client := getTestClient(c)
	createdAt := time.Now().AddDate(0, 0, -3)
	updatedAt := createdAt.AddDate(0, 0, 1)
	stores := []openfga.Store{{
		Id:        openfga.PtrString("1"),
		Name:      openfga.PtrString("TestStore1"),
		CreatedAt: &createdAt,
		UpdatedAt: &updatedAt,
	}, {
		Id:        openfga.PtrString("2"),
		Name:      openfga.PtrString("TestStore2"),
		CreatedAt: &createdAt,
		UpdatedAt: &updatedAt,
	}}

	tests := []struct {
		about             string
		pageSize          int32
		continuationToken string
		mockRoutes        []*RouteResponder
		expectedStores    []openfga.Store
		expectedErr       string
	}{{
		about: "error returned by the client is returned to the caller",
		mockRoutes: []*RouteResponder{{
			route:              ListStore,
			mockResponseStatus: http.StatusInternalServerError,
		}},
		expectedErr: "cannot list stores.*",
	}, {
		about: "stores are listed successfully",
		mockRoutes: []*RouteResponder{{
			route: ListStore,
			mockResponse: openfga.ListStoresResponse{
				Stores: &stores,
			},
		}},
		expectedStores: stores,
	}}

	for _, test := range tests {
		test := test
		c.Run(test.about, func(c *qt.C) {
			// Set up and configure mock http responders.
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()
			for _, mr := range test.mockRoutes {
				httpmock.RegisterResponder(mr.route.method, mr.route.endpoint, mr.generate())
			}

			// Execute the test
			stores, err := client.ListStores(ctx, test.pageSize, test.continuationToken)

			if test.expectedErr != "" {
				c.Assert(err, qt.ErrorMatches, test.expectedErr)
				c.Assert(stores, qt.DeepEquals, []openfga.Store(nil))
			} else {
				c.Assert(err, qt.IsNil)
				c.Assert(stores, qt.DeepEquals, test.expectedStores)
			}

			// Validate that the mock routes were called as expected.
			for _, mr := range test.mockRoutes {
				mr.finish(c)
			}
		})
	}
}

func TestClient_ReadChanges(t *testing.T) {
	c := qt.New(t)

	ctx := context.Background()
	client := getTestClient(c)
	writeOp := openfga.WRITE
	timestamp := time.Now()
	changes := []openfga.TupleChange{{
		TupleKey: &openfga.TupleKey{
			User:     openfga.PtrString(entityTestUser.String()),
			Relation: openfga.PtrString(relationEditor.String()),
			Object:   openfga.PtrString(entityTestContract.String()),
		},
		Operation: &writeOp,
		Timestamp: &timestamp,
	}}

	tests := []struct {
		about             string
		entityType        string
		pageSize          int32
		continuationToken string
		mockRoutes        []*RouteResponder
		expectedChanges   openfga.ReadChangesResponse
		expectedErr       string
	}{{
		about: "error returned by the client is returned to the caller",
		mockRoutes: []*RouteResponder{{
			route:              ReadChanges,
			mockResponseStatus: http.StatusInternalServerError,
		}},
		expectedErr: "cannot read changes.*",
	}, {
		about:      "changes are read successfully",
		entityType: entityTestContract.Kind.String(), // TODO validate in query params
		mockRoutes: []*RouteResponder{{
			route: ReadChanges,
			mockResponse: openfga.ReadChangesResponse{
				Changes:           &changes,
				ContinuationToken: openfga.PtrString("ABC"),
			},
		}},
		expectedChanges: openfga.ReadChangesResponse{
			Changes:           &changes,
			ContinuationToken: openfga.PtrString("ABC"),
		},
	}}

	for _, test := range tests {
		test := test
		c.Run(test.about, func(c *qt.C) {
			// Set up and configure mock http responders.
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()
			for _, mr := range test.mockRoutes {
				httpmock.RegisterResponder(mr.route.method, mr.route.endpoint, mr.generate())
			}

			// Execute the test
			changesResponse, err := client.ReadChanges(ctx, test.entityType, test.pageSize, test.continuationToken)

			if test.expectedErr != "" {
				c.Assert(err, qt.ErrorMatches, test.expectedErr)
				c.Assert(changesResponse, qt.DeepEquals, openfga.ReadChangesResponse{})
			} else {
				c.Assert(err, qt.IsNil)
				c.Assert(changesResponse, qt.DeepEquals, test.expectedChanges)
			}

			// Validate that the mock routes were called as expected.
			for _, mr := range test.mockRoutes {
				mr.finish(c)
			}
		})
	}
}

func TestAuthModelFromJson(t *testing.T) {
	c := qt.New(t)

	tests := []struct {
		about             string
		authModelJson     []byte
		expectedAuthModel []openfga.TypeDefinition
		expectedErr       string
	}{{
		about:         "conversion fails if json is improperly formatted",
		authModelJson: []byte("definitions\": [\n    {\n      \"type\": \"user\",\n      \"relat}"),
		expectedErr:   "cannot unmarshal JSON auth model:.*",
	}, {
		about:         "conversion fails if json does not have a `type_definitions` property",
		authModelJson: []byte("{\n  \"type_defs\": [\n    {\n      \"type\": \"user\",\n      \"relations\": {},\n      \"metadata\": null\n    },\n    {\n      \"type\": \"document\",\n      \"relations\": {\n        \"viewer\": {\n          \"union\": {\n            \"child\": [\n              {\n                \"this\": {}\n              },\n              {\n                \"computedUserset\": {\n                  \"object\": \"\",\n                  \"relation\": \"writer\"\n                }\n              }\n            ]\n          }\n        },\n        \"writer\": {\n          \"this\": {}\n        }\n      },\n      \"metadata\": {\n        \"relations\": {\n          \"viewer\": {\n            \"directly_related_user_types\": [\n              {\n                \"type\": \"user\"\n              }\n            ]\n          },\n          \"writer\": {\n            \"directly_related_user_types\": [\n              {\n                \"type\": \"user\"\n              }\n            ]\n          }\n        }\n      }\n    }\n  ],\n  \"schema_version\": \"1.1\"\n}"),
		expectedErr:   "JSON auth model does not have the \"type_definitions\" property",
	}, {
		about:             "conversion is successful",
		authModelJson:     authModelJson,
		expectedAuthModel: authModel,
	}}

	for _, test := range tests {
		test := test
		c.Run(test.about, func(c *qt.C) {
			// Execute the test
			model, err := ofga.AuthModelFromJson(test.authModelJson)

			if test.expectedErr != "" {
				c.Assert(err, qt.ErrorMatches, test.expectedErr)
				c.Assert(model, qt.DeepEquals, []openfga.TypeDefinition(nil))
			} else {
				c.Assert(err, qt.IsNil)
				c.Assert(model, qt.DeepEquals, test.expectedAuthModel)
			}
		})
	}
}

func TestClient_CreateAuthModel(t *testing.T) {
	c := qt.New(t)

	ctx := context.Background()
	client := getTestClient(c)

	tests := []struct {
		about               string
		authModel           []openfga.TypeDefinition
		mockRoutes          []*RouteResponder
		expectedAuthModelID string
		expectedErr         string
	}{{
		about:     "error returned by the client is returned to the caller",
		authModel: authModel,
		mockRoutes: []*RouteResponder{{
			route:              WriteAuthModel,
			mockResponseStatus: http.StatusInternalServerError,
		}},
		expectedErr: "cannot create auth model.*",
	}, {
		about:     "auth model is created successfully",
		authModel: authModel,
		mockRoutes: []*RouteResponder{{
			route: WriteAuthModel,
			expectedRequest: &openfga.WriteAuthorizationModelRequest{
				TypeDefinitions: authModel,
			},
			mockResponse: openfga.WriteAuthorizationModelResponse{AuthorizationModelId: openfga.PtrString("XYZ")},
		}},
		expectedAuthModelID: "XYZ",
	}}

	for _, test := range tests {
		test := test
		c.Run(test.about, func(c *qt.C) {
			// Set up and configure mock http responders.
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()
			for _, mr := range test.mockRoutes {
				httpmock.RegisterResponder(mr.route.method, mr.route.endpoint, mr.generate())
			}

			// Execute the test
			authModelID, err := client.CreateAuthModel(ctx, test.authModel)

			if test.expectedErr != "" {
				c.Assert(err, qt.ErrorMatches, test.expectedErr)
				c.Assert(authModelID, qt.Equals, "")
			} else {
				c.Assert(err, qt.IsNil)
				c.Assert(authModelID, qt.Equals, test.expectedAuthModelID)
			}

			// Validate that the mock routes were called as expected.
			for _, mr := range test.mockRoutes {
				mr.finish(c)
			}
		})
	}
}

func TestClient_ListAuthModels(t *testing.T) {
	c := qt.New(t)

	ctx := context.Background()
	client := getTestClient(c)

	authModelsResp := []openfga.AuthorizationModel{{
		Id:              openfga.PtrString("12345"),
		SchemaVersion:   "1.1",
		TypeDefinitions: &authModel,
	}}

	tests := []struct {
		about              string
		pageSize           int32
		continuationToken  string
		mockRoutes         []*RouteResponder
		expectedAuthModels []openfga.AuthorizationModel
		expectedErr        string
	}{{
		about: "error returned by the client is returned to the caller",
		mockRoutes: []*RouteResponder{{
			route:              ReadAuthModels,
			mockResponseStatus: http.StatusInternalServerError,
		}},
		expectedErr: "cannot list authorization models.*",
	}, {
		about: "auth models are listed successfully",
		mockRoutes: []*RouteResponder{{
			route: ReadAuthModels,
			mockResponse: openfga.ReadAuthorizationModelsResponse{
				AuthorizationModels: &authModelsResp,
			},
		}},
		expectedAuthModels: authModelsResp,
	}}

	for _, test := range tests {
		test := test
		c.Run(test.about, func(c *qt.C) {
			// Set up and configure mock http responders.
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()
			for _, mr := range test.mockRoutes {
				httpmock.RegisterResponder(mr.route.method, mr.route.endpoint, mr.generate())
			}

			// Execute the test
			authModels, err := client.ListAuthModels(ctx, test.pageSize, test.continuationToken)

			if test.expectedErr != "" {
				c.Assert(err, qt.ErrorMatches, test.expectedErr)
				c.Assert(authModels, qt.DeepEquals, []openfga.AuthorizationModel(nil))
			} else {
				c.Assert(err, qt.IsNil)
				c.Assert(authModels, qt.DeepEquals, test.expectedAuthModels)
			}

			// Validate that the mock routes were called as expected.
			for _, mr := range test.mockRoutes {
				mr.finish(c)
			}
		})
	}
}

func TestClient_GetAuthModel(t *testing.T) {
	c := qt.New(t)

	ctx := context.Background()
	client := getTestClient(c)

	authModelResp := openfga.AuthorizationModel{
		Id:              openfga.PtrString("12345"),
		SchemaVersion:   "1.1",
		TypeDefinitions: &authModel,
	}

	tests := []struct {
		about             string
		authModelID       string
		mockRoutes        []*RouteResponder
		expectedAuthModel openfga.AuthorizationModel
		expectedErr       string
	}{{
		about:       "error returned by the client is returned to the caller",
		authModelID: FGAParams.AuthModelID,
		mockRoutes: []*RouteResponder{{
			route:              ReadAuthModel,
			mockResponseStatus: http.StatusInternalServerError,
		}},
		expectedErr: "cannot list authorization models.*",
	}, {
		about:       "auth model is returned successfully",
		authModelID: FGAParams.AuthModelID,
		mockRoutes: []*RouteResponder{{
			route: ReadAuthModel,
			mockResponse: openfga.ReadAuthorizationModelResponse{
				AuthorizationModel: &authModelResp,
			},
		}},
		expectedAuthModel: authModelResp,
	}}

	for _, test := range tests {
		test := test
		c.Run(test.about, func(c *qt.C) {
			// Set up and configure mock http responders.
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()
			for _, mr := range test.mockRoutes {
				httpmock.RegisterResponder(mr.route.method, mr.route.endpoint, mr.generate())
			}

			// Execute the test
			model, err := client.GetAuthModel(ctx, test.authModelID)

			if test.expectedErr != "" {
				c.Assert(err, qt.ErrorMatches, test.expectedErr)
				c.Assert(model, qt.DeepEquals, openfga.AuthorizationModel{})
			} else {
				c.Assert(err, qt.IsNil)
				c.Assert(model, qt.DeepEquals, test.expectedAuthModel)
			}

			// Validate that the mock routes were called as expected.
			for _, mr := range test.mockRoutes {
				mr.finish(c)
			}
		})
	}
}
