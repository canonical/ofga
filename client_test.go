// Copyright 2023 Canonical Ltd.
// Licensed under the LGPL license, see LICENSE file for details.

// Strategy:
// Due to the design of the underlying openfga client, there is no direct way
// to test the integration of this wrapper library with the underlying client.
//
// However, we can test this integration indirectly by using http mocks.
// We know that our ofga wrapper communicates with the openfga client, which in
// turn uses a http client to talk to the actual openfga instance, i.e.
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
//		client receives the expected requests.
//  - having the mock http client respond with mock responses and ensuring that
//		the wrapper receives the expected responses.

package ofga_test

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/jarcoal/httpmock"
	openfga "github.com/openfga/go-sdk"

	"github.com/canonical/ofga"
	"github.com/canonical/ofga/mockhttp"
)

var (
	CheckRoute          = mockhttp.Route{Method: http.MethodPost, Endpoint: `=~/stores/(\w+)/check\z`}
	CreateStoreRoute    = mockhttp.Route{Method: http.MethodPost, Endpoint: "/stores"}
	ExpandRoute         = mockhttp.Route{Method: http.MethodPost, Endpoint: `=~/stores/(\w+)/expand\z`}
	GetStoreRoute       = mockhttp.Route{Method: http.MethodGet, Endpoint: `=~/stores/(\w+)\z`}
	ListObjectsRoute    = mockhttp.Route{Method: http.MethodPost, Endpoint: `=~/stores/(\w+)/list-objects\z`}
	ListStoreRoute      = mockhttp.Route{Method: http.MethodGet, Endpoint: "/stores"}
	ReadRoute           = mockhttp.Route{Method: http.MethodPost, Endpoint: `=~/stores/(\w+)/read\z`}
	ReadAuthModelRoute  = mockhttp.Route{Method: http.MethodGet, Endpoint: `=~/stores/(\w+)/authorization-models/(\w+)\z`}
	ReadAuthModelsRoute = mockhttp.Route{Method: http.MethodGet, Endpoint: `=~/stores/(\w+)/authorization-models\z`}
	ReadChangesRoute    = mockhttp.Route{Method: http.MethodGet, Endpoint: `=~/stores/(\w+)/changes\z`}
	WriteRoute          = mockhttp.Route{Method: http.MethodPost, Endpoint: `=~/stores/(\w+)/write\z`}
	WriteAuthModelRoute = mockhttp.Route{Method: http.MethodPost, Endpoint: `=~/stores/(\w+)/authorization-models\z`}
)

var validFGAParams = ofga.OpenFGAParams{
	Scheme:      "http",
	Host:        "localhost",
	Port:        "8080",
	Token:       "InsecureTokenDoNotUse",
	StoreID:     "0TEST000000000000000000000",
	AuthModelID: "TestAuthModelID",
}

// getTestClient creates and returns an ofga.Client. It takes care of setting
// up all the mock http routes required by the client during the initialization
// process.
func getTestClient(c *qt.C) *ofga.Client {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	clientCreationRoutes := []*mockhttp.RouteResponder{{
		Route: ListStoreRoute,
	}, {
		Route: GetStoreRoute,
		MockResponse: openfga.GetStoreResponse{
			Id:   validFGAParams.StoreID,
			Name: "Test Store",
		},
	}, {
		Route: ReadAuthModelRoute,
		MockResponse: openfga.ReadAuthorizationModelResponse{AuthorizationModel: &openfga.AuthorizationModel{
			Id:            validFGAParams.AuthModelID,
			SchemaVersion: "1.1",
		}},
	}}

	for _, cr := range clientCreationRoutes {
		httpmock.RegisterResponder(cr.Route.Method, cr.Route.Endpoint, cr.Generate())
	}

	// Create a client.
	newClient, err := ofga.NewClient(context.Background(), validFGAParams)
	c.Assert(err, qt.IsNil)
	c.Assert(newClient.AuthModelID(), qt.Equals, validFGAParams.AuthModelID)

	for _, cr := range clientCreationRoutes {
		cr.Finish(c)
	}
	return newClient
}

func TestNewClient(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	tests := []struct {
		about               string
		params              ofga.OpenFGAParams
		mockRoutes          []*mockhttp.RouteResponder
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
		expectedErr: "invalid OpenFGA configuration: missing host",
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
		expectedErr: "invalid OpenFGA configuration: missing port",
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
		expectedErr: "invalid OpenFGA configuration: AuthModelID specified without a StoreID",
	}, {
		about: "client creation fails when any other configuration issue occurs (such as passing an invalid scheme)",
		params: ofga.OpenFGAParams{
			Scheme:      "invalidScheme",
			Host:        "localhost",
			Port:        "8080",
			Token:       "InsecureTokenDoNotUse",
			StoreID:     "TestStoreID",
			AuthModelID: "TestAuthModelID",
		},
		expectedErr: "invalid OpenFGA configuration: .*",
	}, {
		about:  "client creation fails when we are unable to list stores from openFGA",
		params: validFGAParams,
		mockRoutes: []*mockhttp.RouteResponder{{
			Route:              ListStoreRoute,
			MockResponseStatus: http.StatusInternalServerError,
		}},
		expectedErr: "cannot list stores.*",
	}, {
		about:  "client creation fails when StoreID is specified but the Get Store request returns an error",
		params: validFGAParams,
		mockRoutes: []*mockhttp.RouteResponder{{
			Route: ListStoreRoute,
		}, {
			Route:              GetStoreRoute,
			MockResponseStatus: http.StatusInternalServerError,
		}},
		expectedErr: "cannot retrieve store.*",
	}, {
		about:  "client creation fails when AuthModelID is specified but the Read Authorization Model request returns an error",
		params: validFGAParams,
		mockRoutes: []*mockhttp.RouteResponder{{
			Route: ListStoreRoute,
		}, {
			Route:        GetStoreRoute,
			MockResponse: openfga.GetStoreResponse{Name: "Test Store"},
		}, {
			Route:              ReadAuthModelRoute,
			MockResponseStatus: http.StatusInternalServerError,
		}},
		expectedErr: "cannot retrieve authModel.*",
	}, {
		about:  "client created successfully",
		params: validFGAParams,
		mockRoutes: []*mockhttp.RouteResponder{{
			Route: ListStoreRoute,
		}, {
			Route:              GetStoreRoute,
			ExpectedPathParams: []string{validFGAParams.StoreID},
			MockResponse:       openfga.GetStoreResponse{Name: "Test Store"},
		}, {
			Route:              ReadAuthModelRoute,
			ExpectedPathParams: []string{validFGAParams.StoreID, validFGAParams.AuthModelID},
			MockResponse: openfga.ReadAuthorizationModelResponse{
				AuthorizationModel: &openfga.AuthorizationModel{
					Id: validFGAParams.AuthModelID,
				},
			},
		}},
		expectedAuthModelID: validFGAParams.AuthModelID,
	}}

	for _, test := range tests {
		test := test
		c.Run(test.about, func(c *qt.C) {
			// Set up and configure the http mocks.
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()
			for _, mr := range test.mockRoutes {
				httpmock.RegisterResponder(mr.Route.Method, mr.Route.Endpoint, mr.Generate())
			}

			// Execute the test.
			client, err := ofga.NewClient(ctx, test.params)

			if test.expectedErr != "" {
				c.Assert(err, qt.ErrorMatches, test.expectedErr)
				c.Assert(client, qt.IsNil)
			} else {
				c.Assert(err, qt.IsNil)
				c.Assert(client.AuthModelID(), qt.Equals, test.expectedAuthModelID)
			}

			// Validate that the mock routes were called as expected.
			for _, mr := range test.mockRoutes {
				mr.Finish(c)
			}
		})
	}
}

func TestClientUpdateStoreIDAndAuthModelID(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	tests := []struct {
		about string

		mockRoutes []*mockhttp.RouteResponder

		tuples            []ofga.Tuple
		updateStoreID     string
		updateAuthModelID string
	}{{
		about: "success: no configuration change",
		tuples: []ofga.Tuple{
			{
				Object:   &entityTestUser,
				Relation: relationEditor,
				Target:   &entityTestContract,
			},
		},
		mockRoutes: []*mockhttp.RouteResponder{{
			Route:              WriteRoute,
			ExpectedPathParams: []string{validFGAParams.StoreID},
			ExpectedReqBody: openfga.WriteRequest{
				Writes: openfga.NewWriteRequestWrites([]openfga.TupleKey{{
					User:     entityTestUser.String(),
					Relation: relationEditor.String(),
					Object:   entityTestContract.String(),
				}}),
				AuthorizationModelId: openfga.PtrString(validFGAParams.AuthModelID),
			},
		}},
	}, {
		about:         "success: storeID changed",
		updateStoreID: "1TEST111111111111111111111",
		tuples: []ofga.Tuple{
			{
				Object:   &entityTestUser,
				Relation: relationEditor,
				Target:   &entityTestContract,
			},
		},
		mockRoutes: []*mockhttp.RouteResponder{{
			Route:              WriteRoute,
			ExpectedPathParams: []string{"1TEST111111111111111111111"},
			ExpectedReqBody: openfga.WriteRequest{
				Writes: openfga.NewWriteRequestWrites([]openfga.TupleKey{{
					User:     entityTestUser.String(),
					Relation: relationEditor.String(),
					Object:   entityTestContract.String(),
				}}),
				AuthorizationModelId: openfga.PtrString(validFGAParams.AuthModelID),
			},
		}},
	}, {
		about:             "success: authModelID changed",
		updateAuthModelID: "AuthModel3000",
		tuples: []ofga.Tuple{
			{
				Object:   &entityTestUser,
				Relation: relationEditor,
				Target:   &entityTestContract,
			},
		},
		mockRoutes: []*mockhttp.RouteResponder{{
			Route:              WriteRoute,
			ExpectedPathParams: []string{validFGAParams.StoreID},
			ExpectedReqBody: openfga.WriteRequest{
				Writes: openfga.NewWriteRequestWrites([]openfga.TupleKey{{
					User:     entityTestUser.String(),
					Relation: relationEditor.String(),
					Object:   entityTestContract.String(),
				}}),
				AuthorizationModelId: openfga.PtrString("AuthModel3000"),
			},
		}},
	}, {
		about:             "success: storeID and authModelID changed",
		updateStoreID:     "1TEST111111111111111111111",
		updateAuthModelID: "AuthModel3000",
		tuples: []ofga.Tuple{
			{
				Object:   &entityTestUser,
				Relation: relationEditor,
				Target:   &entityTestContract,
			},
		},
		mockRoutes: []*mockhttp.RouteResponder{{
			Route:              WriteRoute,
			ExpectedPathParams: []string{"1TEST111111111111111111111"},
			ExpectedReqBody: openfga.WriteRequest{
				Writes: openfga.NewWriteRequestWrites([]openfga.TupleKey{{
					User:     entityTestUser.String(),
					Relation: relationEditor.String(),
					Object:   entityTestContract.String(),
				}}),
				AuthorizationModelId: openfga.PtrString("AuthModel3000"),
			},
		}},
	}}
	for _, test := range tests {
		test := test
		c.Run(test.about, func(c *qt.C) {
			client := getTestClient(c)

			// Set up and configure mock http responders.
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()
			for _, mr := range test.mockRoutes {
				httpmock.RegisterResponder(mr.Route.Method, mr.Route.Endpoint, mr.Generate())
			}

			// Execute the test.
			if test.updateStoreID != "" {
				client.SetStoreID(test.updateStoreID)
				c.Assert(client.StoreID(), qt.Equals, test.updateStoreID)
			}
			if test.updateAuthModelID != "" {
				client.SetAuthModelID(test.updateAuthModelID)
				c.Assert(client.AuthModelID(), qt.Equals, test.updateAuthModelID)
			}
			err := client.AddRelation(ctx, test.tuples...)
			c.Assert(err, qt.IsNil)

			// Validate that the mock routes were called as expected.
			for _, mr := range test.mockRoutes {
				mr.Finish(c)
			}
		})
	}
}

func TestClientAddRelation(t *testing.T) {
	c := qt.New(t)

	ctx := context.Background()
	client := getTestClient(c)

	tests := []struct {
		about       string
		tuples      []ofga.Tuple
		mockRoutes  []*mockhttp.RouteResponder
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
		mockRoutes: []*mockhttp.RouteResponder{{
			Route:              WriteRoute,
			MockResponseStatus: http.StatusBadRequest,
		}},
		expectedErr: "cannot add or remove relations.*",
	}, {
		about: "relation added successfully",
		tuples: []ofga.Tuple{
			{
				Object:   &entityTestUser,
				Relation: relationEditor,
				Target:   &entityTestContract,
			},
		},
		mockRoutes: []*mockhttp.RouteResponder{{
			Route:              WriteRoute,
			ExpectedPathParams: []string{validFGAParams.StoreID},
			ExpectedReqBody: openfga.WriteRequest{
				Writes: openfga.NewWriteRequestWrites([]openfga.TupleKey{{
					User:     entityTestUser.String(),
					Relation: relationEditor.String(),
					Object:   entityTestContract.String(),
				}}),
				AuthorizationModelId: openfga.PtrString(validFGAParams.AuthModelID),
			},
		}},
	}, {
		about: "wildcard relation added successfully",
		tuples: []ofga.Tuple{
			{
				Object:   &publicEntityUser,
				Relation: relationEditor,
				Target:   &entityTestContract,
			},
		},
		mockRoutes: []*mockhttp.RouteResponder{{
			Route:              WriteRoute,
			ExpectedPathParams: []string{validFGAParams.StoreID},
			ExpectedReqBody: openfga.WriteRequest{
				Writes: openfga.NewWriteRequestWrites([]openfga.TupleKey{{
					User:     publicEntityUser.String(),
					Relation: relationEditor.String(),
					Object:   entityTestContract.String(),
				}}),
				AuthorizationModelId: openfga.PtrString(validFGAParams.AuthModelID),
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
				httpmock.RegisterResponder(mr.Route.Method, mr.Route.Endpoint, mr.Generate())
			}

			// Execute the test.
			err := client.AddRelation(ctx, test.tuples...)

			if test.expectedErr != "" {
				c.Assert(err, qt.ErrorMatches, test.expectedErr)
			} else {
				c.Assert(err, qt.IsNil)
			}

			// Validate that the mock routes were called as expected.
			for _, mr := range test.mockRoutes {
				mr.Finish(c)
			}
		})
	}
}

func TestClientCheckRelationMethods(t *testing.T) {
	c := qt.New(t)

	ctx := context.Background()
	client := getTestClient(c)

	tests := []struct {
		about            string
		function         func(context.Context, ofga.Tuple, ...ofga.Tuple) (bool, error)
		tuple            ofga.Tuple
		contextualTuples []ofga.Tuple
		mockRoutes       []*mockhttp.RouteResponder
		expectedAllowed  bool
		expectedErr      string
	}{{
		about:    "error returned by the client is returned to the caller",
		function: client.CheckRelation,
		tuple: ofga.Tuple{
			Object:   &entityTestUser,
			Relation: relationEditor,
			Target:   &entityTestContract,
		},
		mockRoutes: []*mockhttp.RouteResponder{{
			Route:              CheckRoute,
			MockResponseStatus: http.StatusInternalServerError,
		}},
		expectedErr: "cannot check relation.*",
	}, {
		about:    "relation checked successfully and allowed returned as true",
		function: client.CheckRelation,
		tuple: ofga.Tuple{
			Object:   &entityTestUser,
			Relation: relationEditor,
			Target:   &entityTestContract,
		},
		mockRoutes: []*mockhttp.RouteResponder{{
			Route: CheckRoute,
			ExpectedReqBody: openfga.CheckRequest{
				TupleKey: openfga.CheckRequestTupleKey{
					User:     entityTestUser.String(),
					Relation: relationEditor.String(),
					Object:   entityTestContract.String(),
				},
				AuthorizationModelId: openfga.PtrString(validFGAParams.AuthModelID),
				Trace:                openfga.PtrBool(false),
			},
			MockResponse: openfga.CheckResponse{
				Allowed: openfga.PtrBool(true),
			},
		}},
		expectedAllowed: true,
	}, {
		about:    "relation checked successfully and allowed returned as false",
		function: client.CheckRelation,
		tuple: ofga.Tuple{
			Object:   &entityTestUser,
			Relation: relationEditor,
			Target:   &entityTestContract,
		},
		mockRoutes: []*mockhttp.RouteResponder{{
			Route:              CheckRoute,
			ExpectedPathParams: []string{validFGAParams.StoreID},
			ExpectedReqBody: openfga.CheckRequest{
				TupleKey: openfga.CheckRequestTupleKey{
					User:     entityTestUser.String(),
					Relation: relationEditor.String(),
					Object:   entityTestContract.String(),
				},
				AuthorizationModelId: openfga.PtrString(validFGAParams.AuthModelID),
				Trace:                openfga.PtrBool(false),
			},
			MockResponse: openfga.CheckResponse{
				Allowed: openfga.PtrBool(false),
			},
		}},
		expectedAllowed: false,
	}, {
		about:    "relation checked successfully with contextual tuples",
		function: client.CheckRelation,
		tuple: ofga.Tuple{
			Object:   &entityTestUser,
			Relation: relationEditor,
			Target:   &entityTestContract,
		},
		contextualTuples: []ofga.Tuple{
			{
				Object:   &entityTestUser2,
				Relation: relationEditor,
				Target:   &entityTestContract,
			},
		},
		mockRoutes: []*mockhttp.RouteResponder{{
			Route:              CheckRoute,
			ExpectedPathParams: []string{validFGAParams.StoreID},
			ExpectedReqBody: openfga.CheckRequest{
				TupleKey: openfga.CheckRequestTupleKey{
					User:     entityTestUser.String(),
					Relation: relationEditor.String(),
					Object:   entityTestContract.String(),
				},
				AuthorizationModelId: openfga.PtrString(validFGAParams.AuthModelID),
				ContextualTuples: &openfga.ContextualTupleKeys{
					TupleKeys: []openfga.TupleKey{{
						User:     entityTestUser2.String(),
						Relation: relationEditor.String(),
						Object:   entityTestContract.String(),
					}},
				},
				Trace: openfga.PtrBool(false),
			},
			MockResponse: openfga.CheckResponse{
				Allowed: openfga.PtrBool(false),
			},
		}},
		expectedAllowed: false,
	}, {
		about:    "error returned by the client is returned to the caller (with tracing)",
		function: client.CheckRelationWithTracing,
		tuple: ofga.Tuple{
			Object:   &entityTestUser,
			Relation: relationEditor,
			Target:   &entityTestContract,
		},
		mockRoutes: []*mockhttp.RouteResponder{{
			Route:              CheckRoute,
			MockResponseStatus: http.StatusInternalServerError,
		}},
		expectedErr: "cannot check relation.*",
	}, {
		about:    "relation checked successfully and allowed returned as true (with tracing)",
		function: client.CheckRelationWithTracing,
		tuple: ofga.Tuple{
			Object:   &entityTestUser,
			Relation: relationEditor,
			Target:   &entityTestContract,
		},
		mockRoutes: []*mockhttp.RouteResponder{{
			Route: CheckRoute,
			ExpectedReqBody: openfga.CheckRequest{
				TupleKey: openfga.CheckRequestTupleKey{
					User:     entityTestUser.String(),
					Relation: relationEditor.String(),
					Object:   entityTestContract.String(),
				},
				AuthorizationModelId: openfga.PtrString(validFGAParams.AuthModelID),
				Trace:                openfga.PtrBool(true),
			},
			MockResponse: openfga.CheckResponse{
				Allowed: openfga.PtrBool(true),
			},
		}},
		expectedAllowed: true,
	}, {
		about:    "relation checked successfully and allowed returned as false (with tracing)",
		function: client.CheckRelationWithTracing,
		tuple: ofga.Tuple{
			Object:   &entityTestUser,
			Relation: relationEditor,
			Target:   &entityTestContract,
		},
		mockRoutes: []*mockhttp.RouteResponder{{
			Route:              CheckRoute,
			ExpectedPathParams: []string{validFGAParams.StoreID},
			ExpectedReqBody: openfga.CheckRequest{
				TupleKey: openfga.CheckRequestTupleKey{
					User:     entityTestUser.String(),
					Relation: relationEditor.String(),
					Object:   entityTestContract.String(),
				},
				AuthorizationModelId: openfga.PtrString(validFGAParams.AuthModelID),
				Trace:                openfga.PtrBool(true),
			},
			MockResponse: openfga.CheckResponse{
				Allowed: openfga.PtrBool(false),
			},
		}},
		expectedAllowed: false,
	}, {
		about:    "relation checked successfully with contextual tuples (with tracing)",
		function: client.CheckRelationWithTracing,
		tuple: ofga.Tuple{
			Object:   &entityTestUser,
			Relation: relationEditor,
			Target:   &entityTestContract,
		},
		contextualTuples: []ofga.Tuple{
			{
				Object:   &entityTestUser2,
				Relation: relationEditor,
				Target:   &entityTestContract,
			},
		},
		mockRoutes: []*mockhttp.RouteResponder{{
			Route:              CheckRoute,
			ExpectedPathParams: []string{validFGAParams.StoreID},
			ExpectedReqBody: openfga.CheckRequest{
				TupleKey: openfga.CheckRequestTupleKey{
					User:     entityTestUser.String(),
					Relation: relationEditor.String(),
					Object:   entityTestContract.String(),
				},
				AuthorizationModelId: openfga.PtrString(validFGAParams.AuthModelID),
				ContextualTuples: &openfga.ContextualTupleKeys{
					TupleKeys: []openfga.TupleKey{{
						User:     entityTestUser2.String(),
						Relation: relationEditor.String(),
						Object:   entityTestContract.String(),
					}},
				},
				Trace: openfga.PtrBool(true),
			},
			MockResponse: openfga.CheckResponse{
				Allowed: openfga.PtrBool(false),
			},
		}},
		expectedAllowed: false,
	}}

	for _, test := range tests {
		test := test
		c.Run(test.about, func(c *qt.C) {
			// Set up and configure mock http responders.
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()
			for _, mr := range test.mockRoutes {
				httpmock.RegisterResponder(mr.Route.Method, mr.Route.Endpoint, mr.Generate())
			}

			// Execute the test.
			allowed, err := test.function(ctx, test.tuple, test.contextualTuples...)

			if test.expectedErr != "" {
				c.Assert(err, qt.ErrorMatches, test.expectedErr)
			} else {
				c.Assert(err, qt.IsNil)
				c.Assert(allowed, qt.Equals, test.expectedAllowed)
			}

			// Validate that the mock routes were called as expected.
			for _, mr := range test.mockRoutes {
				mr.Finish(c)
			}
		})
	}
}

func TestClientRemoveRelation(t *testing.T) {
	c := qt.New(t)

	ctx := context.Background()
	client := getTestClient(c)

	tests := []struct {
		about       string
		tuples      []ofga.Tuple
		mockRoutes  []*mockhttp.RouteResponder
		expectedErr string
	}{{
		about: "error returned by the client is returned to the caller",
		tuples: []ofga.Tuple{{
			Object:   &entityTestUser,
			Relation: relationEditor,
			Target:   &entityTestContract,
		}},
		mockRoutes: []*mockhttp.RouteResponder{{
			Route:              WriteRoute,
			MockResponseStatus: http.StatusInternalServerError,
		}},
		expectedErr: "cannot add or remove relation.*",
	}, {
		about: "relation removed successfully",
		tuples: []ofga.Tuple{{
			Object:   &entityTestUser,
			Relation: relationEditor,
			Target:   &entityTestContract,
		}},
		mockRoutes: []*mockhttp.RouteResponder{{
			Route:              WriteRoute,
			ExpectedPathParams: []string{validFGAParams.StoreID},
			ExpectedReqBody: openfga.WriteRequest{
				Deletes: openfga.NewWriteRequestDeletes([]openfga.TupleKeyWithoutCondition{{
					User:     entityTestUser.String(),
					Relation: relationEditor.String(),
					Object:   entityTestContract.String(),
				}}),
				AuthorizationModelId: openfga.PtrString(validFGAParams.AuthModelID),
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
				httpmock.RegisterResponder(mr.Route.Method, mr.Route.Endpoint, mr.Generate())
			}

			// Execute the test.
			err := client.RemoveRelation(ctx, test.tuples...)

			if test.expectedErr != "" {
				c.Assert(err, qt.ErrorMatches, test.expectedErr)
			} else {
				c.Assert(err, qt.IsNil)
			}

			// Validate that the mock routes were called as expected.
			for _, mr := range test.mockRoutes {
				mr.Finish(c)
			}
		})
	}
}

func TestClientAddRemoveRelations(t *testing.T) {
	c := qt.New(t)

	ctx := context.Background()
	client := getTestClient(c)

	tests := []struct {
		about        string
		addTuples    []ofga.Tuple
		removeTuples []ofga.Tuple
		mockRoutes   []*mockhttp.RouteResponder
		expectedErr  string
	}{{
		about: "error returned by the client is returned to the caller",
		addTuples: []ofga.Tuple{{
			Object:   &entityTestUser,
			Relation: relationEditor,
			Target:   &entityTestContract,
		}},
		removeTuples: []ofga.Tuple{{
			Object:   &entityTestUser,
			Relation: relationViewer,
			Target:   &entityTestContract,
		}},
		mockRoutes: []*mockhttp.RouteResponder{{
			Route:              WriteRoute,
			MockResponseStatus: http.StatusInternalServerError,
		}},
		expectedErr: "cannot add or remove relations.*",
	}, {
		about: "relations added and removed successfully",
		addTuples: []ofga.Tuple{{
			Object:   &entityTestUser,
			Relation: relationEditor,
			Target:   &entityTestContract,
		}},
		removeTuples: []ofga.Tuple{{
			Object:   &entityTestUser,
			Relation: relationViewer,
			Target:   &entityTestContract,
		}},
		mockRoutes: []*mockhttp.RouteResponder{{
			Route:              WriteRoute,
			ExpectedPathParams: []string{validFGAParams.StoreID},
			ExpectedReqBody: openfga.WriteRequest{
				Writes: openfga.NewWriteRequestWrites([]openfga.TupleKey{{
					User:     entityTestUser.String(),
					Relation: relationEditor.String(),
					Object:   entityTestContract.String(),
				}}),
				Deletes: openfga.NewWriteRequestDeletes([]openfga.TupleKeyWithoutCondition{{
					User:     entityTestUser.String(),
					Relation: relationViewer.String(),
					Object:   entityTestContract.String(),
				}}),
				AuthorizationModelId: openfga.PtrString(validFGAParams.AuthModelID),
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
				httpmock.RegisterResponder(mr.Route.Method, mr.Route.Endpoint, mr.Generate())
			}

			// Execute the test.
			err := client.AddRemoveRelations(ctx, test.addTuples, test.removeTuples)

			if test.expectedErr != "" {
				c.Assert(err, qt.ErrorMatches, test.expectedErr)
			} else {
				c.Assert(err, qt.IsNil)
			}

			// Validate that the mock routes were called as expected.
			for _, mr := range test.mockRoutes {
				mr.Finish(c)
			}
		})
	}
}

func TestClientCreateStore(t *testing.T) {
	c := qt.New(t)

	ctx := context.Background()
	client := getTestClient(c)

	tests := []struct {
		about           string
		storeName       string
		mockRoutes      []*mockhttp.RouteResponder
		expectedStoreID string
		expectedErr     string
	}{{
		about:     "error returned by the client is returned to the caller",
		storeName: "TestStore",
		mockRoutes: []*mockhttp.RouteResponder{{
			Route:              CreateStoreRoute,
			ExpectedReqBody:    openfga.CreateStoreRequest{Name: "TestStore"},
			MockResponseStatus: http.StatusInternalServerError,
		}},
		expectedErr: "cannot create store.*",
	}, {
		about:     "store is created successfully",
		storeName: "TestStore",
		mockRoutes: []*mockhttp.RouteResponder{{
			Route:           CreateStoreRoute,
			ExpectedReqBody: openfga.CreateStoreRequest{Name: "TestStore"},
			MockResponse:    openfga.CreateStoreResponse{Id: "12345"},
		}},
		expectedStoreID: "12345",
	}}

	for _, test := range tests {
		test := test
		c.Run(test.about, func(c *qt.C) {
			// Set up and configure mock http responders.
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()
			for _, mr := range test.mockRoutes {
				httpmock.RegisterResponder(mr.Route.Method, mr.Route.Endpoint, mr.Generate())
			}

			// Execute the test.
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
				mr.Finish(c)
			}
		})
	}
}

func TestClientListStores(t *testing.T) {
	c := qt.New(t)

	ctx := context.Background()
	client := getTestClient(c)
	createdAt := time.Now().AddDate(0, 0, -3)
	updatedAt := createdAt.AddDate(0, 0, 1)
	stores := []openfga.Store{{
		Id:        "1",
		Name:      "TestStore1",
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}, {
		Id:        "2",
		Name:      "TestStore2",
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}}

	tests := []struct {
		about             string
		pageSize          int32
		continuationToken string
		mockRoutes        []*mockhttp.RouteResponder
		expectedStores    []openfga.Store
		expectedNextToken string
		expectedErr       string
	}{{
		about: "error returned by the client is returned to the caller",
		mockRoutes: []*mockhttp.RouteResponder{{
			Route:              ListStoreRoute,
			MockResponseStatus: http.StatusInternalServerError,
		}},
		expectedErr: "cannot list stores.*",
	}, {
		about:             "stores are listed successfully",
		pageSize:          25,
		continuationToken: "SimulatedToken",
		mockRoutes: []*mockhttp.RouteResponder{{
			Route: ListStoreRoute,
			ExpectedReqQueryParams: url.Values{
				"page_size":          []string{"25"},
				"continuation_token": []string{"SimulatedToken"},
			},
			MockResponse: openfga.ListStoresResponse{
				Stores:            stores,
				ContinuationToken: "NextToken",
			},
		}},
		expectedNextToken: "NextToken",
		expectedStores:    stores,
	}}

	for _, test := range tests {
		test := test
		c.Run(test.about, func(c *qt.C) {
			// Set up and configure mock http responders.
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()
			for _, mr := range test.mockRoutes {
				httpmock.RegisterResponder(mr.Route.Method, mr.Route.Endpoint, mr.Generate())
			}

			// Execute the test.
			lsr, err := client.ListStores(ctx, test.pageSize, test.continuationToken)

			if test.expectedErr != "" {
				c.Assert(err, qt.ErrorMatches, test.expectedErr)
				c.Assert(len(lsr.GetStores()), qt.Equals, 0)
				c.Assert(lsr.GetContinuationToken(), qt.Equals, "")
			} else {
				c.Assert(err, qt.IsNil)
				c.Assert(lsr.GetStores(), qt.DeepEquals, test.expectedStores)
				c.Assert(lsr.GetContinuationToken(), qt.Equals, test.expectedNextToken)
			}

			// Validate that the mock routes were called as expected.
			for _, mr := range test.mockRoutes {
				mr.Finish(c)
			}
		})
	}
}

func TestClientReadChanges(t *testing.T) {
	c := qt.New(t)

	ctx := context.Background()
	client := getTestClient(c)
	writeOp := openfga.WRITE
	timestamp := time.Now()
	changes := []openfga.TupleChange{{
		TupleKey: openfga.TupleKey{
			User:     entityTestUser.String(),
			Relation: relationEditor.String(),
			Object:   entityTestContract.String(),
		},
		Operation: writeOp,
		Timestamp: timestamp,
	}}

	tests := []struct {
		about             string
		entityType        string
		pageSize          int32
		continuationToken string
		mockRoutes        []*mockhttp.RouteResponder
		expectedResponse  openfga.ReadChangesResponse
		expectedErr       string
	}{{
		about: "error returned by the client is returned to the caller",
		mockRoutes: []*mockhttp.RouteResponder{{
			Route:              ReadChangesRoute,
			MockResponseStatus: http.StatusInternalServerError,
		}},
		expectedErr: "cannot read changes.*",
	}, {
		about:             "changes are read successfully",
		entityType:        entityTestContract.Kind.String(),
		pageSize:          25,
		continuationToken: "SimulatedToken",
		mockRoutes: []*mockhttp.RouteResponder{{
			Route:              ReadChangesRoute,
			ExpectedPathParams: []string{validFGAParams.StoreID},
			ExpectedReqQueryParams: url.Values{
				"page_size":          []string{"25"},
				"continuation_token": []string{"SimulatedToken"},
				"type":               []string{entityTestContract.Kind.String()},
			},
			MockResponse: openfga.ReadChangesResponse{
				Changes:           changes,
				ContinuationToken: openfga.PtrString("NextToken"),
			},
		}},
		expectedResponse: openfga.ReadChangesResponse{
			Changes:           changes,
			ContinuationToken: openfga.PtrString("NextToken"),
		},
	}}

	for _, test := range tests {
		test := test
		c.Run(test.about, func(c *qt.C) {
			// Set up and configure mock http responders.
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()
			for _, mr := range test.mockRoutes {
				httpmock.RegisterResponder(mr.Route.Method, mr.Route.Endpoint, mr.Generate())
			}

			// Execute the test.
			changesResponse, err := client.ReadChanges(ctx, test.entityType, test.pageSize, test.continuationToken)

			if test.expectedErr != "" {
				c.Assert(err, qt.ErrorMatches, test.expectedErr)
				c.Assert(changesResponse, qt.DeepEquals, openfga.ReadChangesResponse{})
			} else {
				c.Assert(err, qt.IsNil)
				c.Assert(changesResponse, qt.DeepEquals, test.expectedResponse)
			}

			// Validate that the mock routes were called as expected.
			for _, mr := range test.mockRoutes {
				mr.Finish(c)
			}
		})
	}
}

func TestAuthModelFromJson(t *testing.T) {
	c := qt.New(t)

	tests := []struct {
		about             string
		authModelJson     []byte
		expectedAuthModel *openfga.AuthorizationModel
		expectedErr       string
	}{{
		about:         "conversion fails if input is not a valid json",
		authModelJson: []byte(`"definitions": [{"type": "user"}`),
		expectedErr:   "cannot unmarshal JSON auth model:.*",
	}, {
		about: "conversion fails if json does not have a `type_definitions` property",
		authModelJson: []byte(`{
		  "wrong_top_level_key": [
			{
			  "type": "user",
			  "relations": {},
			  "metadata": null
			}
		  ],
		  "schema_version": "1.1"
		}`),
		expectedErr: `"type_definitions" field not found`,
	}, {
		about: "conversion fails if `type_definitions` are specified in an incorrect format (using numbers for type instead of string)",
		authModelJson: []byte(`{
		  "type_definitions": [
			{
			  "type": 1,
			  "relations": {},
			  "metadata": null
			}
		  ],
		  "schema_version": "1.1"
		}`),
		expectedErr: "cannot unmarshal JSON auth model.*",
	}, {
		about:             "conversion is successful",
		authModelJson:     authModelJson,
		expectedAuthModel: &authModel,
	}}

	for _, test := range tests {
		test := test
		c.Run(test.about, func(c *qt.C) {
			// Execute the test.
			model, err := ofga.AuthModelFromJSON(test.authModelJson)

			if test.expectedErr != "" {
				c.Assert(err, qt.ErrorMatches, test.expectedErr)
				c.Assert(model, qt.IsNil)
			} else {
				c.Assert(err, qt.IsNil)
				c.Assert(model, qt.DeepEquals, test.expectedAuthModel)
			}
		})
	}
}

func TestClientCreateAuthModel(t *testing.T) {
	c := qt.New(t)

	ctx := context.Background()
	client := getTestClient(c)

	tests := []struct {
		about               string
		authModel           *openfga.AuthorizationModel
		mockRoutes          []*mockhttp.RouteResponder
		expectedAuthModelID string
		expectedErr         string
	}{{
		about:     "error returned by the client is returned to the caller",
		authModel: &authModel,
		mockRoutes: []*mockhttp.RouteResponder{{
			Route:              WriteAuthModelRoute,
			MockResponseStatus: http.StatusInternalServerError,
		}},
		expectedErr: "cannot create auth model.*",
	}, {
		about:     "auth model is created successfully",
		authModel: &authModel,
		mockRoutes: []*mockhttp.RouteResponder{{
			Route:              WriteAuthModelRoute,
			ExpectedPathParams: []string{validFGAParams.StoreID},
			ExpectedReqBody: &openfga.WriteAuthorizationModelRequest{
				TypeDefinitions: authModel.TypeDefinitions,
				SchemaVersion:   authModel.SchemaVersion,
			},
			MockResponse: openfga.WriteAuthorizationModelResponse{AuthorizationModelId: "XYZ"},
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
				httpmock.RegisterResponder(mr.Route.Method, mr.Route.Endpoint, mr.Generate())
			}

			// Execute the test.
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
				mr.Finish(c)
			}
		})
	}
}

func TestClientListAuthModels(t *testing.T) {
	c := qt.New(t)

	ctx := context.Background()
	client := getTestClient(c)

	authModelsResp := []openfga.AuthorizationModel{{
		Id:              "12345",
		SchemaVersion:   authModel.SchemaVersion,
		TypeDefinitions: authModel.TypeDefinitions,
	}}

	tests := []struct {
		about             string
		pageSize          int32
		continuationToken string
		mockRoutes        []*mockhttp.RouteResponder
		expectedResponse  openfga.ReadAuthorizationModelsResponse
		expectedErr       string
	}{{
		about: "error returned by the client is returned to the caller",
		mockRoutes: []*mockhttp.RouteResponder{{
			Route:              ReadAuthModelsRoute,
			MockResponseStatus: http.StatusInternalServerError,
		}},
		expectedErr: "cannot list authorization models.*",
	}, {
		about:             "auth models are listed successfully",
		pageSize:          25,
		continuationToken: "SimulatedToken",
		mockRoutes: []*mockhttp.RouteResponder{{
			Route:              ReadAuthModelsRoute,
			ExpectedPathParams: []string{validFGAParams.StoreID},
			ExpectedReqQueryParams: url.Values{
				"page_size":          []string{"25"},
				"continuation_token": []string{"SimulatedToken"},
			},
			MockResponse: openfga.ReadAuthorizationModelsResponse{
				AuthorizationModels: authModelsResp,
				ContinuationToken:   openfga.PtrString("NextToken"),
			},
		}},
		expectedResponse: openfga.ReadAuthorizationModelsResponse{
			AuthorizationModels: authModelsResp,
			ContinuationToken:   openfga.PtrString("NextToken"),
		},
	}}

	for _, test := range tests {
		test := test
		c.Run(test.about, func(c *qt.C) {
			// Set up and configure mock http responders.
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()
			for _, mr := range test.mockRoutes {
				httpmock.RegisterResponder(mr.Route.Method, mr.Route.Endpoint, mr.Generate())
			}

			// Execute the test.
			modelsResponse, err := client.ListAuthModels(ctx, test.pageSize, test.continuationToken)

			if test.expectedErr != "" {
				c.Assert(err, qt.ErrorMatches, test.expectedErr)
				c.Assert(modelsResponse, qt.DeepEquals, openfga.ReadAuthorizationModelsResponse{})
			} else {
				c.Assert(err, qt.IsNil)
				c.Assert(modelsResponse, qt.DeepEquals, test.expectedResponse)
			}

			// Validate that the mock routes were called as expected.
			for _, mr := range test.mockRoutes {
				mr.Finish(c)
			}
		})
	}
}

func TestClientGetAuthModel(t *testing.T) {
	c := qt.New(t)

	ctx := context.Background()
	client := getTestClient(c)

	authModelResp := openfga.AuthorizationModel{
		Id:              "12345",
		SchemaVersion:   authModel.SchemaVersion,
		TypeDefinitions: authModel.TypeDefinitions,
	}

	tests := []struct {
		about             string
		authModelID       string
		mockRoutes        []*mockhttp.RouteResponder
		expectedAuthModel openfga.AuthorizationModel
		expectedErr       string
	}{{
		about:       "error returned by the client is returned to the caller",
		authModelID: validFGAParams.AuthModelID,
		mockRoutes: []*mockhttp.RouteResponder{{
			Route:              ReadAuthModelRoute,
			MockResponseStatus: http.StatusInternalServerError,
		}},
		expectedErr: "cannot list authorization models.*",
	}, {
		about:       "auth model is returned successfully",
		authModelID: validFGAParams.AuthModelID,
		mockRoutes: []*mockhttp.RouteResponder{{
			Route:              ReadAuthModelRoute,
			ExpectedPathParams: []string{validFGAParams.StoreID, validFGAParams.AuthModelID},
			MockResponse: openfga.ReadAuthorizationModelResponse{
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
				httpmock.RegisterResponder(mr.Route.Method, mr.Route.Endpoint, mr.Generate())
			}

			// Execute the test.
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
				mr.Finish(c)
			}
		})
	}
}

func TestValidateTupleForFindMatchingTuples(t *testing.T) {
	c := qt.New(t)

	tests := []struct {
		about       string
		tuple       ofga.Tuple
		expectedErr string
	}{{
		about: "error when Target does not specify Kind",
		tuple: ofga.Tuple{
			Object:   &ofga.Entity{Kind: "user", ID: "XYZ"},
			Relation: "member",
			Target:   &ofga.Entity{ID: "123"},
		},
		expectedErr: "missing tuple.Target.Kind",
	}, {
		about: "error when Target ID is missing and Object is not fully specified",
		tuple: ofga.Tuple{
			Object:   &ofga.Entity{Kind: "user"},
			Relation: "member",
			Target:   &ofga.Entity{Kind: "organization"},
		},
		expectedErr: "either tuple.Target.ID or tuple.Object must be specified",
	}, {
		about: "error when Target Relation is specified",
		tuple: ofga.Tuple{
			Object:   &ofga.Entity{Kind: "user", ID: "XYZ"},
			Relation: "member",
			Target:   &ofga.Entity{Kind: "organization", ID: "123", Relation: "admin"},
		},
		expectedErr: "tuple.Target.Relation must not be set",
	}, {
		about: "no error when tuple is specified in correct format",
		tuple: ofga.Tuple{
			Object:   &ofga.Entity{Kind: "user", ID: "XYZ"},
			Relation: "member",
			Target:   &ofga.Entity{Kind: "organization", ID: "123"},
		},
	}}

	for _, test := range tests {
		test := test
		c.Run(test.about, func(c *qt.C) {
			// Execute the test.
			err := ofga.ValidateTupleForFindMatchingTuples(test.tuple)

			if test.expectedErr != "" {
				c.Assert(err, qt.ErrorMatches, test.expectedErr)
			} else {
				c.Assert(err, qt.IsNil)
			}
		})
	}
}

func TestClientFindMatchingTuples(t *testing.T) {
	c := qt.New(t)

	ctx := context.Background()
	client := getTestClient(c)

	now := time.Now()
	future := now.AddDate(0, 0, 1)

	readTuples := []openfga.Tuple{{
		Key:       openfga.TupleKey{User: "user:abc", Relation: "member", Object: "organization:123"},
		Timestamp: now,
	}, {
		Key:       openfga.TupleKey{User: "user:xyz", Relation: "member", Object: "organization:123"},
		Timestamp: future,
	}}

	readConvertedTuples := []ofga.TimestampedTuple{{
		Tuple: ofga.Tuple{
			Object:   &ofga.Entity{Kind: "user", ID: "abc"},
			Relation: "member",
			Target:   &ofga.Entity{Kind: "organization", ID: "123"},
		},
		Timestamp: now,
	}, {
		Tuple: ofga.Tuple{
			Object:   &ofga.Entity{Kind: "user", ID: "xyz"},
			Relation: "member",
			Target:   &ofga.Entity{Kind: "organization", ID: "123"},
		},
		Timestamp: future,
	}}

	tests := []struct {
		about                     string
		tuple                     ofga.Tuple
		pageSize                  int32
		continuationToken         string
		mockRoutes                []*mockhttp.RouteResponder
		expectedTuples            []ofga.TimestampedTuple
		expectedContinuationToken string
		expectedErr               string
	}{{
		about: "passing in an invalid tuple for the Read API returns an error",
		tuple: ofga.Tuple{
			Object:   &ofga.Entity{Kind: "user", ID: "XYZ"},
			Relation: "member",
			Target:   &ofga.Entity{ID: "123"},
		},
		expectedErr: "invalid tuple for FindMatchingTuples.*",
	}, {
		about: "error raised by the underlying client is returned to the caller",
		mockRoutes: []*mockhttp.RouteResponder{{
			Route:              ReadRoute,
			MockResponseStatus: http.StatusInternalServerError,
		}},
		expectedErr: "cannot fetch matching tuples.*",
	}, {
		about: "an error converting a response tuple is raised to the caller",
		tuple: ofga.Tuple{},
		mockRoutes: []*mockhttp.RouteResponder{{
			Route:           ReadRoute,
			ExpectedReqBody: openfga.ReadRequest{},
			MockResponse: openfga.ReadResponse{
				Tuples: []openfga.Tuple{{
					Key:       openfga.TupleKey{User: "userabc", Relation: "member", Object: "organization:123"},
					Timestamp: now,
				}},
			},
		}},
		expectedErr: "cannot parse tuple.*",
	}, {
		about: "passing in an empty tuple returns all tuples in the system",
		tuple: ofga.Tuple{},
		mockRoutes: []*mockhttp.RouteResponder{{
			Route:           ReadRoute,
			ExpectedReqBody: openfga.ReadRequest{},
			MockResponse: openfga.ReadResponse{
				Tuples:            readTuples,
				ContinuationToken: "NextToken",
			},
		}},
		expectedTuples:            readConvertedTuples,
		expectedContinuationToken: "NextToken",
	}, {
		about: "passing in a valid tuple for the Read API returns matching tuples in the system",
		tuple: ofga.Tuple{
			Object:   &ofga.Entity{Kind: "user", ID: "XYZ"},
			Relation: "member",
			Target:   &ofga.Entity{Kind: "organization", ID: "123"},
		},
		continuationToken: "SimulatedToken",
		pageSize:          50,
		mockRoutes: []*mockhttp.RouteResponder{{
			Route:              ReadRoute,
			ExpectedPathParams: []string{validFGAParams.StoreID},
			ExpectedReqBody: openfga.ReadRequest{
				TupleKey:          &openfga.ReadRequestTupleKey{User: openfga.PtrString("user:XYZ"), Relation: openfga.PtrString("member"), Object: openfga.PtrString("organization:123")},
				PageSize:          openfga.PtrInt32(50),
				ContinuationToken: openfga.PtrString("SimulatedToken"),
			},
			MockResponse: openfga.ReadResponse{
				Tuples: readTuples,
			},
		}},
		expectedTuples: readConvertedTuples,
	}}

	for _, test := range tests {
		test := test
		c.Run(test.about, func(c *qt.C) {
			// Set up and configure mock http responders.
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()
			for _, mr := range test.mockRoutes {
				httpmock.RegisterResponder(mr.Route.Method, mr.Route.Endpoint, mr.Generate())
			}

			// Execute the test.
			tuples, cToken, err := client.FindMatchingTuples(ctx, test.tuple, test.pageSize, test.continuationToken)

			if test.expectedErr != "" {
				c.Assert(err, qt.ErrorMatches, test.expectedErr)
				c.Assert(tuples, qt.IsNil)
				c.Assert(cToken, qt.Equals, "")
			} else {
				c.Assert(err, qt.IsNil)
				c.Assert(tuples, qt.DeepEquals, test.expectedTuples)
				c.Assert(cToken, qt.Equals, test.expectedContinuationToken)
			}

			// Validate that the mock routes were called as expected.
			for _, mr := range test.mockRoutes {
				mr.Finish(c)
			}
		})
	}
}

func TestValidateTupleForFindUsersByRelation(t *testing.T) {
	c := qt.New(t)

	tests := []struct {
		about       string
		tuple       ofga.Tuple
		expectedErr string
	}{{
		about: "error when Target does not specify Kind",
		tuple: ofga.Tuple{
			Relation: "member",
			Target:   &ofga.Entity{ID: "123"},
		},
		expectedErr: "missing tuple.Target",
	}, {
		about: "error when Target does not specify ID",
		tuple: ofga.Tuple{
			Relation: "member",
			Target:   &ofga.Entity{Kind: "organization"},
		},
		expectedErr: "missing tuple.Target",
	}, {
		about: "error when Target specifies a relation",
		tuple: ofga.Tuple{
			Relation: "member",
			Target:   &ofga.Entity{Kind: "organization", ID: "123", Relation: "admins"},
		},
		expectedErr: "tuple.Target.Relation must not be set",
	}, {
		about: "error when tuple.Relation is not specified",
		tuple: ofga.Tuple{
			Relation: "",
			Target:   &ofga.Entity{Kind: "organization", ID: "123"},
		},
		expectedErr: "missing tuple.Relation",
	}, {
		about: "no error when tuple is specified correctly",
		tuple: ofga.Tuple{
			Relation: "member",
			Target:   &ofga.Entity{Kind: "organization", ID: "123"},
		},
	}}

	for _, test := range tests {
		test := test
		c.Run(test.about, func(c *qt.C) {
			// Execute the test.
			err := ofga.ValidateTupleForFindUsersByRelation(test.tuple)

			if test.expectedErr != "" {
				c.Assert(err, qt.ErrorMatches, test.expectedErr)
			} else {
				c.Assert(err, qt.IsNil)
			}
		})
	}
}

func TestClientFindUsersByRelation(t *testing.T) {
	c := qt.New(t)

	ctx := context.Background()
	client := getTestClient(c)

	tests := []struct {
		about         string
		tuple         ofga.Tuple
		maxDepth      int
		mockRoutes    []*mockhttp.RouteResponder
		expectedUsers []ofga.Entity
		expectedErr   string
	}{{
		about: "passing in a maxDepth of less than 1 results in an error",
		tuple: ofga.Tuple{
			Relation: "",
			Target:   &ofga.Entity{Kind: "organization", ID: "123"},
		},
		maxDepth:    0,
		expectedErr: "maxDepth must be greater than or equal to 1",
	}, {
		about: "passing in an invalid tuple for the Expand API returns an error",
		tuple: ofga.Tuple{
			Relation: "",
			Target:   &ofga.Entity{Kind: "organization", ID: "123"},
		},
		maxDepth:    1,
		expectedErr: "invalid tuple for FindUsersByRelation.*",
	}, {
		about: "error when parsing an incorrectly formatted user entity is raised",
		tuple: ofga.Tuple{
			Relation: "member",
			Target:   &ofga.Entity{Kind: "organization", ID: "123"},
		},
		maxDepth: 1,
		mockRoutes: []*mockhttp.RouteResponder{{
			Route: ExpandRoute,
			MockResponse: openfga.ExpandResponse{
				Tree: &openfga.UsersetTree{
					Root: &openfga.Node{
						Leaf: &openfga.Leaf{
							Users: &openfga.Users{Users: []string{"userXYZ"}},
						},
					},
				},
			},
		}},
		expectedErr: "cannot parse entity .* from Expand response.*",
	}, {
		about: "found users are returned successfully",
		tuple: ofga.Tuple{
			Relation: "member",
			Target:   &ofga.Entity{Kind: "organization", ID: "123"},
		},
		maxDepth: 1,
		mockRoutes: []*mockhttp.RouteResponder{{
			Route:              ExpandRoute,
			ExpectedPathParams: []string{validFGAParams.StoreID},
			ExpectedReqBody: openfga.ExpandRequest{
				TupleKey: openfga.ExpandRequestTupleKey{
					Relation: "member",
					Object:   "organization:123",
				},
				AuthorizationModelId: openfga.PtrString(validFGAParams.AuthModelID),
			},
			MockResponse: openfga.ExpandResponse{
				Tree: &openfga.UsersetTree{
					Root: &openfga.Node{
						Leaf: &openfga.Leaf{
							Users: &openfga.Users{Users: []string{"user:XYZ", "user:ABC"}},
						},
					},
				},
			},
		}},
		expectedUsers: []ofga.Entity{
			{Kind: "user", ID: "XYZ"},
			{Kind: "user", ID: "ABC"},
		},
	}}

	for _, test := range tests {
		test := test
		c.Run(test.about, func(c *qt.C) {
			// Set up and configure mock http responders.
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()
			for _, mr := range test.mockRoutes {
				httpmock.RegisterResponder(mr.Route.Method, mr.Route.Endpoint, mr.Generate())
			}

			// Execute the test.
			users, err := client.FindUsersByRelation(ctx, test.tuple, test.maxDepth)

			if test.expectedErr != "" {
				c.Assert(err, qt.ErrorMatches, test.expectedErr)
				c.Assert(users, qt.IsNil)
			} else {
				c.Assert(err, qt.IsNil)
				c.Assert(users, qt.ContentEquals, test.expectedUsers)
			}

			// Validate that the mock routes were called as expected.
			for _, mr := range test.mockRoutes {
				mr.Finish(c)
			}
		})
	}
}

func TestClientFindUsersByRelationInternal(t *testing.T) {
	c := qt.New(t)

	ctx := context.Background()
	client := getTestClient(c)

	tests := []struct {
		about         string
		tuple         ofga.Tuple
		maxDepth      int
		mockRoutes    []*mockhttp.RouteResponder
		expectedUsers map[string]bool
		expectedErr   string
	}{{
		about: "passing in an invalid tuple for the Expand API returns an error",
		tuple: ofga.Tuple{
			Relation: "",
			Target:   &ofga.Entity{Kind: "organization", ID: "123"},
		},
		maxDepth:    0,
		expectedErr: "invalid tuple for FindUsersByRelation.*",
	}, {
		about: "a maxDepth of 0 causes the function to return the unexpanded result",
		tuple: ofga.Tuple{
			Relation: "member",
			Target:   &ofga.Entity{Kind: "organization", ID: "123"},
		},
		maxDepth: 0,
		expectedUsers: map[string]bool{
			"organization:123#member": true,
		},
	}, {
		about: "error raised by the underlying client is returned to the caller",
		tuple: ofga.Tuple{
			Relation: "member",
			Target:   &ofga.Entity{Kind: "organization", ID: "123"},
		},
		maxDepth: 1,
		mockRoutes: []*mockhttp.RouteResponder{{
			Route:              ExpandRoute,
			MockResponseStatus: http.StatusInternalServerError,
		}},
		expectedErr: "cannot execute Expand request.*",
	}, {
		about: "error due to an invalid tree (without root) being returned is propagated forward",
		tuple: ofga.Tuple{
			Relation: "member",
			Target:   &ofga.Entity{Kind: "organization", ID: "123"},
		},
		maxDepth: 1,
		mockRoutes: []*mockhttp.RouteResponder{{
			Route:        ExpandRoute,
			MockResponse: openfga.ExpandResponse{Tree: &openfga.UsersetTree{Root: nil}},
		}},
		expectedErr: "tree from Expand response has no root",
	}, {
		about: "error expanding intermediate results is propagated forward",
		tuple: ofga.Tuple{
			Relation: "member",
			Target:   &ofga.Entity{Kind: "organization", ID: "123"},
		},
		maxDepth: 1,
		mockRoutes: []*mockhttp.RouteResponder{{
			Route: ExpandRoute,
			MockResponse: openfga.ExpandResponse{
				Tree: &openfga.UsersetTree{
					Root: &openfga.Node{},
				},
			},
		}},
		expectedErr: "cannot expand the intermediate results.*",
	}, {
		about: "found users are returned successfully",
		tuple: ofga.Tuple{
			Relation: "member",
			Target:   &ofga.Entity{Kind: "organization", ID: "123"},
		},
		maxDepth: 1,
		mockRoutes: []*mockhttp.RouteResponder{{
			Route:              ExpandRoute,
			ExpectedPathParams: []string{validFGAParams.StoreID},
			ExpectedReqBody: openfga.ExpandRequest{
				TupleKey: openfga.ExpandRequestTupleKey{
					Relation: "member",
					Object:   "organization:123",
				},
				AuthorizationModelId: openfga.PtrString(validFGAParams.AuthModelID),
			},
			MockResponse: openfga.ExpandResponse{
				Tree: &openfga.UsersetTree{
					Root: &openfga.Node{
						Leaf: &openfga.Leaf{
							Users: &openfga.Users{Users: []string{"user:XYZ", "user:ABC"}},
						},
					},
				},
			},
		}},
		expectedUsers: map[string]bool{
			"user:ABC": true,
			"user:XYZ": true,
		},
	}}

	for _, test := range tests {
		test := test
		c.Run(test.about, func(c *qt.C) {
			// Set up and configure mock http responders.
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()
			for _, mr := range test.mockRoutes {
				httpmock.RegisterResponder(mr.Route.Method, mr.Route.Endpoint, mr.Generate())
			}

			// Execute the test.
			users, err := ofga.FindUsersByRelationInternal(client, ctx, test.tuple, test.maxDepth)

			if test.expectedErr != "" {
				c.Assert(err, qt.ErrorMatches, test.expectedErr)
				c.Assert(users, qt.IsNil)
			} else {
				c.Assert(err, qt.IsNil)
				c.Assert(users, qt.ContentEquals, test.expectedUsers)
			}

			// Validate that the mock routes were called as expected.
			for _, mr := range test.mockRoutes {
				mr.Finish(c)
			}
		})
	}
}

func TestClientTraverseTree(t *testing.T) {
	c := qt.New(t)

	ctx := context.Background()
	client := getTestClient(c)

	tests := []struct {
		about         string
		node          openfga.Node
		maxDepth      int
		mockRoutes    []*mockhttp.RouteResponder
		expectedUsers map[string]bool
		expectedErr   string
	}{{
		about: "union node with an invalid childNode causes an error",
		node: openfga.Node{
			Union: &openfga.Nodes{
				Nodes: []openfga.Node{
					{
						Leaf: &openfga.Leaf{
							Users: &openfga.Users{Users: []string{"user:XYZ"}},
						},
					},
					{},
				},
			},
		},
		maxDepth:    1,
		expectedErr: "unknown node type",
	}, {
		about: "union node is expanded properly",
		node: openfga.Node{
			Union: &openfga.Nodes{
				Nodes: []openfga.Node{{
					Leaf: &openfga.Leaf{
						Users: &openfga.Users{Users: []string{"user:XYZ"}},
					},
				}, {
					Leaf: &openfga.Leaf{
						Users: &openfga.Users{Users: []string{"user:ABC"}},
					},
				}},
			},
		},
		maxDepth: 1,
		expectedUsers: map[string]bool{
			"user:XYZ": true,
			"user:ABC": true,
		},
	}, {
		about: "leaf node without any Users, Computed or TupleToUserSet fields raises an error",
		node: openfga.Node{
			Leaf: &openfga.Leaf{},
		},
		maxDepth:    1,
		expectedErr: "unknown leaf type",
	}, {
		about: "leaf node with improper user representation raises an error",
		node: openfga.Node{
			Leaf: &openfga.Leaf{
				Users: &openfga.Users{Users: []string{"user:XYZ##"}},
			},
		},
		maxDepth:    1,
		expectedErr: "unknown user representation.*",
	}, {
		about: "leaf node with proper user representation returns unexpanded result when maxDepth is zero",
		node: openfga.Node{
			Leaf: &openfga.Leaf{
				Users: &openfga.Users{Users: []string{"organization:123#member"}},
			},
		},
		maxDepth: 0,
		expectedUsers: map[string]bool{
			"organization:123#member": true,
		},
	}, {
		about: "leaf node with proper user representation and maxDepth greater than zero returns expanded result",
		node: openfga.Node{
			Leaf: &openfga.Leaf{
				Users: &openfga.Users{Users: []string{"organization:123#member"}},
			},
		},
		maxDepth: 1,
		mockRoutes: []*mockhttp.RouteResponder{{
			Route: ExpandRoute,
			MockResponse: openfga.ExpandResponse{
				Tree: &openfga.UsersetTree{
					Root: &openfga.Node{
						Leaf: &openfga.Leaf{
							Users: &openfga.Users{Users: []string{"user:ABC", "user:XYZ"}},
						},
					},
				},
			},
		}},
		expectedUsers: map[string]bool{
			"user:ABC": true,
			"user:XYZ": true,
		},
	}, {
		about: "leaf node with computed node returns unexpanded result when maxDepth is zero",
		node: openfga.Node{
			Leaf: &openfga.Leaf{
				Computed: &openfga.Computed{
					Userset: "organization:123#member",
				},
			},
		},
		maxDepth: 0,
		expectedUsers: map[string]bool{
			"organization:123#member": true,
		},
	}, {
		about: "leaf node with computed node returns expanded result when maxDepth is greater than zero",
		node: openfga.Node{
			Leaf: &openfga.Leaf{
				Computed: &openfga.Computed{
					Userset: "organization:123#member",
				},
			},
		},
		maxDepth: 1,
		mockRoutes: []*mockhttp.RouteResponder{{
			Route: ExpandRoute,
			MockResponse: openfga.ExpandResponse{
				Tree: &openfga.UsersetTree{
					Root: &openfga.Node{
						Leaf: &openfga.Leaf{
							Users: &openfga.Users{Users: []string{"user:ABC", "user:XYZ"}},
						},
					},
				},
			},
		}},
		expectedUsers: map[string]bool{
			"user:ABC": true,
			"user:XYZ": true,
		},
	}, {
		about: "leaf node with tupleToUserSet node returns unexpanded result when maxDepth is zero",
		node: openfga.Node{
			Leaf: &openfga.Leaf{
				TupleToUserset: &openfga.UsersetTreeTupleToUserset{
					Computed: []openfga.Computed{{
						Userset: "organization:123#member",
					}},
				},
			},
		},
		maxDepth: 0,
		expectedUsers: map[string]bool{
			"organization:123#member": true,
		},
	}, {
		about: "leaf node with tupleToUserSet node returns expanded result when maxDepth greater than zero",
		node: openfga.Node{
			Leaf: &openfga.Leaf{
				TupleToUserset: &openfga.UsersetTreeTupleToUserset{
					Computed: []openfga.Computed{{
						Userset: "organization:123#member",
					}},
				},
			},
		},
		maxDepth: 1,
		mockRoutes: []*mockhttp.RouteResponder{{
			Route:              ExpandRoute,
			ExpectedPathParams: []string{validFGAParams.StoreID},
			MockResponse: openfga.ExpandResponse{
				Tree: &openfga.UsersetTree{
					Root: &openfga.Node{
						Leaf: &openfga.Leaf{
							Users: &openfga.Users{Users: []string{"user:ABC", "user:XYZ"}},
						},
					},
				},
			},
		}},
		expectedUsers: map[string]bool{
			"user:ABC": true,
			"user:XYZ": true,
		},
	}}

	for _, test := range tests {
		test := test
		c.Run(test.about, func(c *qt.C) {
			// Set up and configure mock http responders.
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()
			for _, mr := range test.mockRoutes {
				httpmock.RegisterResponder(mr.Route.Method, mr.Route.Endpoint, mr.Generate())
			}

			// Execute the test.
			userMap, err := ofga.TraverseTree(client, ctx, &test.node, test.maxDepth)

			if test.expectedErr != "" {
				c.Assert(err, qt.ErrorMatches, test.expectedErr)
				c.Assert(userMap, qt.IsNil)
			} else {
				c.Assert(err, qt.IsNil)
				c.Assert(userMap, qt.ContentEquals, test.expectedUsers)
			}

			// Validate that the mock routes were called as expected.
			for _, mr := range test.mockRoutes {
				mr.Finish(c)
			}
		})
	}
}

func TestClientExpand(t *testing.T) {
	c := qt.New(t)

	ctx := context.Background()
	client := getTestClient(c)

	tests := []struct {
		about         string
		maxDepth      int
		userStrings   []string
		mockRoutes    []*mockhttp.RouteResponder
		expectedUsers map[string]bool
		expectedErr   string
	}{{
		about:         "calling expand on single user returns the user as is",
		maxDepth:      1,
		userStrings:   []string{"user:XYZ"},
		expectedUsers: map[string]bool{"user:XYZ": true},
	}, {
		about:       "calling expand on an unknown user representation string results in an error",
		maxDepth:    1,
		userStrings: []string{"organization:123#member#XYZ"},
		expectedErr: "unknown user representation.*",
	}, {
		about:       "error converting a userString into ofga.Tuple representation is returned to caller",
		maxDepth:    1,
		userStrings: []string{"organization123#member"},
		mockRoutes: []*mockhttp.RouteResponder{{
			Route:              ExpandRoute,
			MockResponseStatus: http.StatusInternalServerError,
		}},
		expectedErr: "failed to parse tuple.*",
	}, {
		about:       "error from expanding a userSet is returned to the caller",
		maxDepth:    1,
		userStrings: []string{"organization:123#member"},
		mockRoutes: []*mockhttp.RouteResponder{{
			Route:              ExpandRoute,
			MockResponseStatus: http.StatusInternalServerError,
		}},
		expectedErr: "failed to expand.*",
	}, {
		about:       "calling expand on a userSet returns the unexpanded results when maxDepth is zero",
		maxDepth:    0,
		userStrings: []string{"organization:123#member"},
		expectedUsers: map[string]bool{
			"organization:123#member": true,
		},
	}, {
		about:       "calling expand on a userSet expands it to the individual users when maxDepth is greater than zero",
		maxDepth:    1,
		userStrings: []string{"organization:123#member"},
		mockRoutes: []*mockhttp.RouteResponder{{
			Route:              ExpandRoute,
			ExpectedPathParams: []string{validFGAParams.StoreID},
			MockResponse: openfga.ExpandResponse{
				Tree: &openfga.UsersetTree{
					Root: &openfga.Node{
						Leaf: &openfga.Leaf{
							Users: &openfga.Users{Users: []string{"user:ABC", "user:XYZ"}},
						},
					},
				},
			},
		}},
		expectedUsers: map[string]bool{
			"user:ABC": true,
			"user:XYZ": true,
		},
	}}

	for _, test := range tests {
		test := test
		c.Run(test.about, func(c *qt.C) {
			// Set up and configure mock http responders.
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()
			for _, mr := range test.mockRoutes {
				httpmock.RegisterResponder(mr.Route.Method, mr.Route.Endpoint, mr.Generate())
			}

			// Execute the test.
			userMap, err := ofga.Expand(client, ctx, test.maxDepth, test.userStrings...)

			if test.expectedErr != "" {
				c.Assert(err, qt.ErrorMatches, test.expectedErr)
				c.Assert(userMap, qt.IsNil)
			} else {
				c.Assert(err, qt.IsNil)
				c.Assert(userMap, qt.ContentEquals, test.expectedUsers)
			}

			// Validate that the mock routes were called as expected.
			for _, mr := range test.mockRoutes {
				mr.Finish(c)
			}
		})
	}
}

func TestClientExpandComputed(t *testing.T) {
	c := qt.New(t)

	ctx := context.Background()
	client := getTestClient(c)

	tests := []struct {
		about         string
		maxDepth      int
		leaf          openfga.Leaf
		computed      []openfga.Computed
		mockRoutes    []*mockhttp.RouteResponder
		expectedUsers map[string]bool
		expectedErr   string
	}{{
		about:       "calling expandComputed on a node without a userSet results in an error",
		maxDepth:    1,
		computed:    []openfga.Computed{{}},
		expectedErr: "missing userSet",
	}, {
		about:    "calling expandComputed on a node with a userSet with an invalid representation results in an error",
		maxDepth: 1,
		computed: []openfga.Computed{{
			Userset: "organization:123#member#admin",
		}},
		mockRoutes: []*mockhttp.RouteResponder{{
			Route:              ExpandRoute,
			MockResponseStatus: http.StatusInternalServerError,
		}},
		expectedErr: "unknown user representation.*",
	}, {
		about:    "calling expandComputed on a node with a userSet returns the unexpanded result when maxDepth is zero",
		maxDepth: 0,
		computed: []openfga.Computed{{
			Userset: "organization:123#member",
		}},
		expectedUsers: map[string]bool{
			"organization:123#member": true,
		},
	}, {
		about:    "calling expandComputed on a node with a userSet expands the userSet when maxDepth is greater than zero",
		maxDepth: 1,
		computed: []openfga.Computed{{
			Userset: "organization:123#member",
		}},
		mockRoutes: []*mockhttp.RouteResponder{{
			Route:              ExpandRoute,
			ExpectedPathParams: []string{validFGAParams.StoreID},
			MockResponse: openfga.ExpandResponse{
				Tree: &openfga.UsersetTree{
					Root: &openfga.Node{
						Leaf: &openfga.Leaf{
							Users: &openfga.Users{Users: []string{"user:ABC", "user:XYZ"}},
						},
					},
				},
			},
		}},
		expectedUsers: map[string]bool{
			"user:ABC": true,
			"user:XYZ": true,
		},
	}}

	for _, test := range tests {
		test := test
		c.Run(test.about, func(c *qt.C) {
			// Set up and configure mock http responders.
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()
			for _, mr := range test.mockRoutes {
				httpmock.RegisterResponder(mr.Route.Method, mr.Route.Endpoint, mr.Generate())
			}

			// Execute the test.
			userMap, err := ofga.ExpandComputed(client, ctx, test.maxDepth, test.leaf, test.computed...)

			if test.expectedErr != "" {
				c.Assert(err, qt.ErrorMatches, test.expectedErr)
				c.Assert(userMap, qt.IsNil)
			} else {
				c.Assert(err, qt.IsNil)
				c.Assert(userMap, qt.ContentEquals, test.expectedUsers)
			}

			// Validate that the mock routes were called as expected.
			for _, mr := range test.mockRoutes {
				mr.Finish(c)
			}
		})
	}
}

func TestValidateTupleForFindAccessibleObjectsByRelation(t *testing.T) {
	c := qt.New(t)

	tests := []struct {
		about       string
		tuple       ofga.Tuple
		expectedErr string
	}{{
		about: "error when tuple.Object.Kind is missing",
		tuple: ofga.Tuple{
			Object: &ofga.Entity{ID: "XYZ"},
		},
		expectedErr: "missing tuple.Object",
	}, {
		about: "error when tuple.Object.ID is missing",
		tuple: ofga.Tuple{
			Object: &ofga.Entity{Kind: "user"},
		},
		expectedErr: "missing tuple.Object",
	}, {
		about: "error when tuple.Relation is missing",
		tuple: ofga.Tuple{
			Object: &ofga.Entity{Kind: "user", ID: "XYZ"},
		},
		expectedErr: "missing tuple.Relation",
	}, {
		about: "error when tuple.Target.Kind is missing",
		tuple: ofga.Tuple{
			Object:   &ofga.Entity{Kind: "user", ID: "XYZ"},
			Relation: "member",
			Target:   &ofga.Entity{},
		},
		expectedErr: "only tuple.Target.Kind must be set",
	}, {
		about: "error when tuple.Target.ID is specified",
		tuple: ofga.Tuple{
			Object:   &ofga.Entity{Kind: "user", ID: "XYZ"},
			Relation: "member",
			Target:   &ofga.Entity{Kind: "organization", ID: "123"},
		},
		expectedErr: "only tuple.Target.Kind must be set",
	}, {
		about: "error when tuple.Target.Relation is specified",
		tuple: ofga.Tuple{
			Object:   &ofga.Entity{Kind: "user", ID: "XYZ"},
			Relation: "member",
			Target:   &ofga.Entity{Kind: "organization", Relation: "admin"},
		},
		expectedErr: "only tuple.Target.Kind must be set",
	}, {
		about: "no error when tuple is specified correctly",
		tuple: ofga.Tuple{
			Object:   &ofga.Entity{Kind: "user", ID: "XYZ"},
			Relation: "member",
			Target:   &ofga.Entity{Kind: "organization"},
		},
	}}

	for _, test := range tests {
		test := test
		c.Run(test.about, func(c *qt.C) {
			// Execute the test.
			err := ofga.ValidateTupleForFindAccessibleObjectsByRelation(test.tuple)

			if test.expectedErr != "" {
				c.Assert(err, qt.ErrorMatches, test.expectedErr)
			} else {
				c.Assert(err, qt.IsNil)
			}
		})
	}
}

func TestClientFindAccessibleObjectsByRelation(t *testing.T) {
	c := qt.New(t)

	ctx := context.Background()
	client := getTestClient(c)

	tests := []struct {
		about            string
		tuple            ofga.Tuple
		contextualTuples []ofga.Tuple
		mockRoutes       []*mockhttp.RouteResponder
		expectedObjects  []ofga.Entity
		expectedErr      string
	}{{
		about: "passing in an invalid tuple for the ListObjects API returns an error",
		tuple: ofga.Tuple{
			Relation: "",
			Object:   &ofga.Entity{Kind: "user", ID: "XYZ"},
		},
		expectedErr: "invalid tuple for FindAccessibleObjectsByRelation.*",
	}, {
		about: "error returned by the underlying client is forwarded to the caller",
		tuple: ofga.Tuple{
			Object:   &ofga.Entity{Kind: "user", ID: "XYZ"},
			Relation: "member",
			Target:   &ofga.Entity{Kind: "organization"},
		},
		mockRoutes: []*mockhttp.RouteResponder{{
			Route: ListObjectsRoute,
			ExpectedReqBody: openfga.ListObjectsRequest{
				AuthorizationModelId: openfga.PtrString(validFGAParams.AuthModelID),
				Type:                 "organization",
				Relation:             "member",
				User:                 "user:XYZ",
			},
			MockResponseStatus: http.StatusInternalServerError,
		}},
		expectedErr: "cannot list objects.*",
	}, {
		about: "error parsing ListObjects response into internal representation is raised to caller",
		tuple: ofga.Tuple{
			Object:   &ofga.Entity{Kind: "user", ID: "XYZ"},
			Relation: "member",
			Target:   &ofga.Entity{Kind: "organization"},
		},
		mockRoutes: []*mockhttp.RouteResponder{{
			Route: ListObjectsRoute,
			ExpectedReqBody: openfga.ListObjectsRequest{
				AuthorizationModelId: openfga.PtrString(validFGAParams.AuthModelID),
				Type:                 "organization",
				Relation:             "member",
				User:                 "user:XYZ",
			},
			MockResponse: openfga.ListObjectsResponse{Objects: []string{"", "organization:123"}},
		}},
		expectedErr: "cannot parse entity .* from ListObjects response.*",
	}, {
		about: "successful response",
		tuple: ofga.Tuple{
			Object:   &ofga.Entity{Kind: "user", ID: "XYZ"},
			Relation: "member",
			Target:   &ofga.Entity{Kind: "organization"},
		},
		contextualTuples: []ofga.Tuple{{
			Object:   &ofga.Entity{Kind: "user", ID: "XYZ"},
			Relation: "member",
			Target:   &ofga.Entity{Kind: "organization", ID: "456"},
		}},
		mockRoutes: []*mockhttp.RouteResponder{{
			Route:              ListObjectsRoute,
			ExpectedPathParams: []string{validFGAParams.StoreID},
			ExpectedReqBody: openfga.ListObjectsRequest{
				AuthorizationModelId: openfga.PtrString(validFGAParams.AuthModelID),
				Type:                 "organization",
				Relation:             "member",
				User:                 "user:XYZ",
				ContextualTuples: &openfga.ContextualTupleKeys{
					TupleKeys: []openfga.TupleKey{{
						User:     "user:XYZ",
						Relation: "member",
						Object:   "organization:456",
					}},
				},
			},
			MockResponse: openfga.ListObjectsResponse{Objects: []string{"organization:456", "organization:123"}},
		}},
		expectedObjects: []ofga.Entity{
			{Kind: "organization", ID: "123"},
			{Kind: "organization", ID: "456"},
		},
	}}

	for _, test := range tests {
		test := test
		c.Run(test.about, func(c *qt.C) {
			// Set up and configure mock http responders.
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()
			for _, mr := range test.mockRoutes {
				httpmock.RegisterResponder(mr.Route.Method, mr.Route.Endpoint, mr.Generate())
			}

			// Execute the test.
			objects, err := client.FindAccessibleObjectsByRelation(ctx, test.tuple, test.contextualTuples...)

			if test.expectedErr != "" {
				c.Assert(err, qt.ErrorMatches, test.expectedErr)
				c.Assert(objects, qt.IsNil)
			} else {
				c.Assert(err, qt.IsNil)
				c.Assert(objects, qt.ContentEquals, test.expectedObjects)
			}

			// Validate that the mock routes were called as expected.
			for _, mr := range test.mockRoutes {
				mr.Finish(c)
			}
		})
	}
}
