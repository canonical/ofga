package ofga_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/canonical/ofga"
	qt "github.com/frankban/quicktest"
	openfga "github.com/openfga/go-sdk"
)

func TestNewClient(t *testing.T) {
	c := qt.New(t)

	completeParams := ofga.OpenFGAParams{
		Scheme:      "http",
		Host:        "localhost",
		Port:        "8080",
		Token:       "InsecureTokenDoNotUse",
		StoreID:     "TestStoreID",
		AuthModelID: "TestAuthModelID",
	}

	tests := []struct {
		about               string
		ctx                 context.Context
		params              ofga.OpenFGAParams
		api                 ofga.OpenFgaApi
		expectedErr         string
		expectedAuthModelID string
	}{
		{
			about: "client creation fails when Host param is missing",
			ctx:   context.Background(),
			params: ofga.OpenFGAParams{
				Scheme:      "http",
				Host:        "",
				Port:        "8080",
				Token:       "InsecureTokenDoNotUse",
				StoreID:     "TestStoreID",
				AuthModelID: "TestAuthModelID",
			},
			api:         &MockOpenFgaApi{},
			expectedErr: "OpenFGA configuration: missing host",
		},
		{
			about: "client creation fails when Port param is missing",
			ctx:   context.Background(),
			params: ofga.OpenFGAParams{
				Scheme:      "http",
				Host:        "localhost",
				Port:        "",
				Token:       "InsecureTokenDoNotUse",
				StoreID:     "TestStoreID",
				AuthModelID: "TestAuthModelID",
			},
			api:         &MockOpenFgaApi{},
			expectedErr: "OpenFGA configuration: missing port",
		},
		{
			about: "client creation fails when AuthModelID is specified without a StoreID",
			ctx:   context.Background(),
			params: ofga.OpenFGAParams{
				Scheme:      "http",
				Host:        "localhost",
				Port:        "8080",
				Token:       "InsecureTokenDoNotUse",
				StoreID:     "",
				AuthModelID: "TestAuthModelID",
			},
			api:         &MockOpenFgaApi{},
			expectedErr: "OpenFGA configuration: AuthModelID specified without a StoreID",
		},
		{
			about: "client creation fails when AuthModelID is specified without a StoreID",
			ctx:   context.Background(),
			params: ofga.OpenFGAParams{
				Scheme:      "http",
				Host:        "localhost",
				Port:        "8080",
				Token:       "InsecureTokenDoNotUse",
				StoreID:     "",
				AuthModelID: "TestAuthModelID",
			},
			api:         &MockOpenFgaApi{},
			expectedErr: "OpenFGA configuration: AuthModelID specified without a StoreID",
		},
		{
			about:  "client creation fails when we are unable to list stores from openFGA",
			ctx:    context.Background(),
			params: completeParams,
			api: &MockOpenFgaApi{
				listStoreResp: MockResponse[openfga.ListStoresResponse]{
					err: errors.New("simulated error while executing List Stores"),
				},
			},
			expectedErr: "cannot list stores.*",
		},
		{
			about:  "client creation fails when we get a non-200 response for a List Stores request to openFGA",
			ctx:    context.Background(),
			params: completeParams,
			api: &MockOpenFgaApi{
				listStoreResp: MockResponse[openfga.ListStoresResponse]{
					httpResp: &http.Response{StatusCode: http.StatusBadRequest},
				},
			},
			expectedErr: "failed to contact the OpenFga server.*",
		},
		{
			about:  "client creation fails when StoreID is specified but the Get Store request returns an error",
			ctx:    context.Background(),
			params: completeParams,
			api: &MockOpenFgaApi{
				listStoreResp: MockResponse[openfga.ListStoresResponse]{
					httpResp: &http.Response{StatusCode: http.StatusOK},
				},
				getStoreResp: MockResponse[openfga.GetStoreResponse]{
					err: errors.New("simulated error while executing Get Store"),
				},
			},
			expectedErr: "cannot retrieve store.*",
		},
		{
			about:  "client creation fails when AuthModelID is specified but the Read Authorization Model request returns an error",
			ctx:    context.Background(),
			params: completeParams,
			api: &MockOpenFgaApi{
				listStoreResp: MockResponse[openfga.ListStoresResponse]{
					httpResp: &http.Response{StatusCode: http.StatusOK},
				},
				getStoreResp: MockResponse[openfga.GetStoreResponse]{
					resp: openfga.GetStoreResponse{Name: openfga.PtrString("Test Store")},
				},
				readAuthModelResp: MockResponse[openfga.ReadAuthorizationModelResponse]{
					err: errors.New("simulated error while executing Read Authorization Model"),
				},
			},
			expectedErr: "cannot retrieve authModel.*",
		},
		{
			about:  "client created successfully",
			ctx:    context.Background(),
			params: completeParams,
			api: &MockOpenFgaApi{
				listStoreResp: MockResponse[openfga.ListStoresResponse]{
					httpResp: &http.Response{StatusCode: http.StatusOK},
				},
				getStoreResp: MockResponse[openfga.GetStoreResponse]{
					resp: openfga.GetStoreResponse{Name: openfga.PtrString("Test Store")},
				},
				readAuthModelResp: MockResponse[openfga.ReadAuthorizationModelResponse]{
					resp: openfga.ReadAuthorizationModelResponse{AuthorizationModel: &openfga.AuthorizationModel{
						Id: openfga.PtrString(completeParams.AuthModelID),
					}},
				},
			},
		},
	}

	for _, test := range tests {
		test := test
		c.Run(test.about, func(c *qt.C) {
			c.Parallel()

			client, err := ofga.NewClientInternalExport(test.ctx, test.params, test.api)

			if test.expectedErr != "" {
				c.Assert(err, qt.ErrorMatches, test.expectedErr)
			} else {
				c.Assert(err, qt.IsNil)
				c.Assert(client.AuthModelId, qt.DeepEquals, test.params.AuthModelID)
			}
		})
	}
}

func TestClient_AddRelation(t *testing.T) {
	c := qt.New(t)

	completeParams := ofga.OpenFGAParams{
		Scheme:      "http",
		Host:        "localhost",
		Port:        "8080",
		Token:       "InsecureTokenDoNotUse",
		StoreID:     "TestStoreID",
		AuthModelID: "TestAuthModelID",
	}
	user := ofga.Entity{
		Kind: "user",
		ID:   "123",
	}
	const Editor ofga.Relation = "editor"
	contract := ofga.Entity{
		Kind: "contract",
		ID:   "789",
	}

	tests := []struct {
		about       string
		ctx         context.Context
		params      ofga.OpenFGAParams
		api         ofga.OpenFgaApi
		tuples      []ofga.Tuple
		expectedErr string
	}{
		{
			about:  "error returned by the client is returned to the caller",
			ctx:    context.Background(),
			params: completeParams,
			api: &MockOpenFgaApi{
				listStoreResp: MockResponse[openfga.ListStoresResponse]{
					httpResp: &http.Response{StatusCode: http.StatusOK},
				},
				getStoreResp: MockResponse[openfga.GetStoreResponse]{
					resp: openfga.GetStoreResponse{Name: openfga.PtrString("Test Store")},
				},
				readAuthModelResp: MockResponse[openfga.ReadAuthorizationModelResponse]{
					resp: openfga.ReadAuthorizationModelResponse{AuthorizationModel: &openfga.AuthorizationModel{
						Id: openfga.PtrString(completeParams.AuthModelID),
					}},
				},
				writeResp: MockResponse[WriteResponse]{
					err: errors.New("simulated error while executing write"),
				},
			},
			tuples: []ofga.Tuple{
				{
					Object:   &user,
					Relation: Editor,
					Target:   &contract,
				},
			},
			expectedErr: "simulated error while executing write",
		},
		{
			about:  "relation added successfully",
			ctx:    context.Background(),
			params: completeParams,
			api: &MockOpenFgaApi{
				listStoreResp: MockResponse[openfga.ListStoresResponse]{
					httpResp: &http.Response{StatusCode: http.StatusOK},
				},
				getStoreResp: MockResponse[openfga.GetStoreResponse]{
					resp: openfga.GetStoreResponse{Name: openfga.PtrString("Test Store")},
				},
				readAuthModelResp: MockResponse[openfga.ReadAuthorizationModelResponse]{
					resp: openfga.ReadAuthorizationModelResponse{AuthorizationModel: &openfga.AuthorizationModel{
						Id: openfga.PtrString(completeParams.AuthModelID),
					}},
				},
				writeResp: MockResponse[WriteResponse]{},
			},
			tuples: []ofga.Tuple{
				{
					Object:   &user,
					Relation: Editor,
					Target:   &contract,
				},
			},
		},
	}

	for _, test := range tests {
		test := test
		c.Run(test.about, func(c *qt.C) {
			c.Parallel()

			client, err := ofga.NewClientInternalExport(test.ctx, test.params, test.api)
			c.Assert(err, qt.IsNil)
			c.Assert(client.AuthModelId, qt.DeepEquals, test.params.AuthModelID)

			err = client.AddRelation(test.ctx, test.tuples...)

			if test.expectedErr != "" {
				c.Assert(err, qt.ErrorMatches, test.expectedErr)
			} else {
				c.Assert(err, qt.IsNil)
			}
		})
	}
}

func TestClient_CheckRelation(t *testing.T) {
	c := qt.New(t)

	completeParams := ofga.OpenFGAParams{
		Scheme:      "http",
		Host:        "localhost",
		Port:        "8080",
		Token:       "InsecureTokenDoNotUse",
		StoreID:     "TestStoreID",
		AuthModelID: "TestAuthModelID",
	}
	user := ofga.Entity{
		Kind: "user",
		ID:   "123",
	}
	const Editor ofga.Relation = "editor"
	contract := ofga.Entity{
		Kind: "contract",
		ID:   "789",
	}

	tests := []struct {
		about           string
		ctx             context.Context
		params          ofga.OpenFGAParams
		api             ofga.OpenFgaApi
		tuple           ofga.Tuple
		expectedErr     string
		expectedAllowed bool
	}{
		{
			about:  "error returned by the client is returned to the caller",
			ctx:    context.Background(),
			params: completeParams,
			api: &MockOpenFgaApi{
				listStoreResp: MockResponse[openfga.ListStoresResponse]{
					httpResp: &http.Response{StatusCode: http.StatusOK},
				},
				getStoreResp: MockResponse[openfga.GetStoreResponse]{
					resp: openfga.GetStoreResponse{Name: openfga.PtrString("Test Store")},
				},
				readAuthModelResp: MockResponse[openfga.ReadAuthorizationModelResponse]{
					resp: openfga.ReadAuthorizationModelResponse{AuthorizationModel: &openfga.AuthorizationModel{
						Id: openfga.PtrString(completeParams.AuthModelID),
					}},
				},
				checkResp: MockResponse[openfga.CheckResponse]{
					err: errors.New("simulated error while executing check"),
				},
			},
			tuple: ofga.Tuple{
				Object:   &user,
				Relation: Editor,
				Target:   &contract,
			},
			expectedErr: "simulated error while executing check",
		},
		{
			about:  "relation checked successfully",
			ctx:    context.Background(),
			params: completeParams,
			api: &MockOpenFgaApi{
				listStoreResp: MockResponse[openfga.ListStoresResponse]{
					httpResp: &http.Response{StatusCode: http.StatusOK},
				},
				getStoreResp: MockResponse[openfga.GetStoreResponse]{
					resp: openfga.GetStoreResponse{Name: openfga.PtrString("Test Store")},
				},
				readAuthModelResp: MockResponse[openfga.ReadAuthorizationModelResponse]{
					resp: openfga.ReadAuthorizationModelResponse{AuthorizationModel: &openfga.AuthorizationModel{
						Id: openfga.PtrString(completeParams.AuthModelID),
					}},
				},
				checkResp: MockResponse[openfga.CheckResponse]{
					resp:     openfga.CheckResponse{Allowed: openfga.PtrBool(true)},
					httpResp: &http.Response{StatusCode: http.StatusOK},
				},
			},
			tuple: ofga.Tuple{
				Object:   &user,
				Relation: Editor,
				Target:   &contract,
			},
			expectedAllowed: true,
		},
	}

	for _, test := range tests {
		test := test
		c.Run(test.about, func(c *qt.C) {
			c.Parallel()

			client, err := ofga.NewClientInternalExport(test.ctx, test.params, test.api)
			c.Assert(err, qt.IsNil)
			c.Assert(client.AuthModelId, qt.DeepEquals, test.params.AuthModelID)

			allowed, err := client.CheckRelation(test.ctx, test.tuple)

			if test.expectedErr != "" {
				c.Assert(err, qt.ErrorMatches, test.expectedErr)
			} else {
				c.Assert(err, qt.IsNil)
				c.Assert(allowed, qt.Equals, test.expectedAllowed)
			}
		})
	}
}

func TestClient_RemoveRelation(t *testing.T) {
	c := qt.New(t)

	completeParams := ofga.OpenFGAParams{
		Scheme:      "http",
		Host:        "localhost",
		Port:        "8080",
		Token:       "InsecureTokenDoNotUse",
		StoreID:     "TestStoreID",
		AuthModelID: "TestAuthModelID",
	}
	user := ofga.Entity{
		Kind: "user",
		ID:   "123",
	}
	const Editor ofga.Relation = "editor"
	contract := ofga.Entity{
		Kind: "contract",
		ID:   "789",
	}

	tests := []struct {
		about       string
		ctx         context.Context
		params      ofga.OpenFGAParams
		api         ofga.OpenFgaApi
		tuple       ofga.Tuple
		expectedErr string
	}{
		{
			about:  "error returned by the client is returned to the caller",
			ctx:    context.Background(),
			params: completeParams,
			api: &MockOpenFgaApi{
				listStoreResp: MockResponse[openfga.ListStoresResponse]{
					httpResp: &http.Response{StatusCode: http.StatusOK},
				},
				getStoreResp: MockResponse[openfga.GetStoreResponse]{
					resp: openfga.GetStoreResponse{Name: openfga.PtrString("Test Store")},
				},
				readAuthModelResp: MockResponse[openfga.ReadAuthorizationModelResponse]{
					resp: openfga.ReadAuthorizationModelResponse{AuthorizationModel: &openfga.AuthorizationModel{
						Id: openfga.PtrString(completeParams.AuthModelID),
					}},
				},
				writeResp: MockResponse[WriteResponse]{
					err: errors.New("simulated error while executing write"),
				},
			},
			tuple: ofga.Tuple{
				Object:   &user,
				Relation: Editor,
				Target:   &contract,
			},
			expectedErr: "simulated error while executing write",
		},
		{
			about:  "relation removed successfully",
			ctx:    context.Background(),
			params: completeParams,
			api: &MockOpenFgaApi{
				listStoreResp: MockResponse[openfga.ListStoresResponse]{
					httpResp: &http.Response{StatusCode: http.StatusOK},
				},
				getStoreResp: MockResponse[openfga.GetStoreResponse]{
					resp: openfga.GetStoreResponse{Name: openfga.PtrString("Test Store")},
				},
				readAuthModelResp: MockResponse[openfga.ReadAuthorizationModelResponse]{
					resp: openfga.ReadAuthorizationModelResponse{AuthorizationModel: &openfga.AuthorizationModel{
						Id: openfga.PtrString(completeParams.AuthModelID),
					}},
				},
				writeResp: MockResponse[WriteResponse]{},
			},
			tuple: ofga.Tuple{
				Object:   &user,
				Relation: Editor,
				Target:   &contract,
			},
		},
	}

	for _, test := range tests {
		test := test
		c.Run(test.about, func(c *qt.C) {
			c.Parallel()

			client, err := ofga.NewClientInternalExport(test.ctx, test.params, test.api)
			c.Assert(err, qt.IsNil)
			c.Assert(client.AuthModelId, qt.DeepEquals, test.params.AuthModelID)

			err = client.RemoveRelation(test.ctx, test.tuple)

			if test.expectedErr != "" {
				c.Assert(err, qt.ErrorMatches, test.expectedErr)
			} else {
				c.Assert(err, qt.IsNil)
			}
		})
	}
}
