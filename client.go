// Copyright 2023 Canonical Ltd.

// Package ofga provides utilities for interacting with an OpenFGA instance.
package ofga

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/juju/zaputil/zapctx"
	openfga "github.com/openfga/go-sdk"
	"github.com/openfga/go-sdk/credentials"
	"go.uber.org/zap"
)

// OpenFGAParams holds parameters needed to connect to the OpenFGA server.
type OpenFGAParams struct {
	Scheme string
	// The host must be specified without the scheme
	// (i.e. `api.fga.example` instead of `https://api.fga.example`).
	Host        string
	Port        string
	Token       string
	StoreID     string
	AuthModelID string
}

// OpenFgaApi defines the methods of the underlying api client that our Client
// depends upon.
type OpenFgaApi interface {
	Check(ctx context.Context) openfga.ApiCheckRequest
	CreateStore(ctx context.Context) openfga.ApiCreateStoreRequest
	GetStore(ctx context.Context) openfga.ApiGetStoreRequest
	ListStores(ctx context.Context) openfga.ApiListStoresRequest
	Read(ctx context.Context) openfga.ApiReadRequest
	ReadAuthorizationModel(ctx context.Context, id string) openfga.ApiReadAuthorizationModelRequest
	ReadAuthorizationModels(ctx context.Context) openfga.ApiReadAuthorizationModelsRequest
	Write(ctx context.Context) openfga.ApiWriteRequest
	WriteAuthorizationModel(ctx context.Context) openfga.ApiWriteAuthorizationModelRequest
}

// Client is a wrapper over the client provided by OpenFGA
// (https://github.com/openfga/go-sdk). The wrapper contains convenient utility
// methods for interacting with OpenFGA. It also ensures that it is able to
// connect to the specified OpenFGA instance, and verifies the existence of a
// Store and AuthorizationModel if such IDs are provided during configuration.
type Client struct {
	api         OpenFgaApi
	AuthModelId string
}

// NewClient returns a wrapped OpenFGA API client ensuring all calls are made
// to the provided authorisation model (id) and returns what is necessary.
func NewClient(ctx context.Context, p OpenFGAParams) (*Client, error) {
	return newClient(ctx, p, nil)
}

// newClient allows passing in a mock api object for testing.
func newClient(ctx context.Context, p OpenFGAParams, api OpenFgaApi) (*Client, error) {
	if p.Host == "" {
		return nil, errors.New("OpenFGA configuration: missing host")
	}
	if p.Port == "" {
		return nil, errors.New("OpenFGA configuration: missing port")
	}
	if p.StoreID == "" && p.AuthModelID != "" {
		return nil, errors.New("OpenFGA configuration: AuthModelID specified without a StoreID")
	}
	zapctx.Info(ctx, "configuring OpenFGA client",
		zap.String("scheme", p.Scheme),
		zap.String("host", p.Host),
		zap.String("port", p.Port),
		zap.String("store", p.StoreID),
	)

	config := openfga.Configuration{
		ApiScheme: p.Scheme,
		ApiHost:   fmt.Sprintf("%s:%s", p.Host, p.Port),
		StoreId:   p.StoreID,
	}
	if p.Token != "" {
		config.Credentials = &credentials.Credentials{
			Method: credentials.CredentialsMethodApiToken,
			Config: &credentials.Config{
				ApiToken: p.Token,
			},
		}
	}
	configuration, err := openfga.NewConfiguration(config)
	if err != nil {
		return nil, err
	}
	if api == nil {
		client := openfga.NewAPIClient(configuration)
		api = client.OpenFgaApi
	}
	_, response, err := api.ListStores(ctx).Execute()
	if err != nil {
		zapctx.Error(ctx, fmt.Sprintf("cannot list stores %q", err))
		return nil, fmt.Errorf("cannot list stores %q", err)
	}
	if response.StatusCode != http.StatusOK {
		// The response body is only used as extra information in the error
		// message, so if an error occurred while trying to read the response
		// body, we can just ignore it.
		var body []byte
		if response.Body != nil {
			body, _ = io.ReadAll(response.Body)
		}
		return nil, fmt.Errorf("failed to contact the OpenFga server: received %v: %s", response.StatusCode, string(body))
	}

	// If StoreID is present, validate that such a store exists.
	if p.StoreID != "" {
		storeResp, _, err := api.GetStore(ctx).Execute()
		if err != nil {
			zapctx.Error(ctx, fmt.Sprintf("cannot retrieve store %q", err))
			return nil, fmt.Errorf("cannot retrieve store %q", err)
		}
		zapctx.Info(ctx, "store found", zap.String("storeName", storeResp.GetName()))
	}

	// If AuthModelID is present, validate that such an AuthModel exists.
	if p.AuthModelID != "" {
		authModelResp, _, err := api.ReadAuthorizationModel(ctx, p.AuthModelID).Execute()
		if err != nil {
			zapctx.Error(ctx, fmt.Sprintf("cannot retrieve authModel %q", err))
			return nil, fmt.Errorf("cannot retrieve authModel %q", err)
		}
		zapctx.Info(ctx, "auth model found", zap.String("authModelID", authModelResp.AuthorizationModel.GetId()))
	}
	return &Client{
		api:         api,
		AuthModelId: p.AuthModelID,
	}, nil
}

// AddRelation adds the specified relation(s) between the objects & targets as
// specified by the given tuples.
func (c *Client) AddRelation(ctx context.Context, tuples ...Tuple) error {
	wr := openfga.NewWriteRequest()
	wr.SetAuthorizationModelId(c.AuthModelId)

	tupleKeys := make([]openfga.TupleKey, len(tuples))
	for i, tuple := range tuples {
		tupleKeys[i] = tuple.toOpenFGATuple()
	}

	keys := openfga.NewTupleKeys(tupleKeys)
	wr.SetWrites(*keys)
	_, _, err := c.api.Write(ctx).Body(*wr).Execute()
	if err != nil {
		return err
	}
	return nil
}

// CheckRelation verifies that the specified relation exists (either directly or
// indirectly) between the object and the target as specified by the tuple.
func (c *Client) CheckRelation(ctx context.Context, tuple Tuple) (bool, error) {
	zapctx.Debug(
		ctx,
		"check request internal",
		zap.String("tuple object", tuple.Object.String()),
		zap.String("tuple relation", tuple.Relation.String()),
		zap.String("tuple target object", tuple.Target.String()),
	)
	cr := openfga.NewCheckRequest(tuple.toOpenFGATuple())
	cr.SetAuthorizationModelId(c.AuthModelId)

	checkResp, httpResp, err := c.api.Check(ctx).Body(*cr).Execute()
	if err != nil {
		return false, err
	}
	allowed := checkResp.GetAllowed()
	zapctx.Debug(ctx, "check request internal resp code", zap.Int("code", httpResp.StatusCode), zap.Bool("allowed", allowed))
	return allowed, nil
}

// RemoveRelation removes the specified relation(s) between the objects &
// targets as specified by the given tuples.
func (c *Client) RemoveRelation(ctx context.Context, tuples ...Tuple) error {
	wr := openfga.NewWriteRequest()
	wr.SetAuthorizationModelId(c.AuthModelId)

	tupleKeys := make([]openfga.TupleKey, len(tuples))
	for i, tuple := range tuples {
		tupleKeys[i] = tuple.toOpenFGATuple()
	}

	keys := openfga.NewTupleKeys(tupleKeys)
	wr.SetDeletes(*keys)
	_, _, err := c.api.Write(ctx).Body(*wr).Execute()
	if err != nil {
		return err
	}
	return nil
}

// CreateStore creates a new store on the openFGA instance and returns its ID.
func (c *Client) CreateStore(ctx context.Context, name string) (string, error) {
	csr := openfga.NewCreateStoreRequest(name)
	resp, _, err := c.api.CreateStore(ctx).Body(*csr).Execute()
	if err != nil {
		return "", fmt.Errorf("cannot list stores %q", err)
	}
	return resp.GetId(), nil
}

// ListStores returns the list of stores present on the openFGA instance.
func (c *Client) ListStores(ctx context.Context, pageSize int32, paginationToken string) ([]openfga.Store, error) {
	lsr := c.api.ListStores(ctx)

	if pageSize != 0 {
		lsr = lsr.PageSize(pageSize)
	}
	if paginationToken != "" {
		lsr = lsr.ContinuationToken(paginationToken)
	}

	resp, _, err := lsr.Execute()
	if err != nil {
		return nil, fmt.Errorf("cannot list stores %q", err)
	}
	return resp.GetStores(), nil
}

// AuthModelFromJson converts the input json representation of an authorization
// model into a slice of TypeDefinitions that can be used with the API.
func AuthModelFromJson(data []byte) ([]openfga.TypeDefinition, error) {
	wrapper := make(map[string]interface{})
	err := json.Unmarshal(data, &wrapper)
	if err != nil {
		return nil, err
	}

	b, err := json.Marshal(wrapper["type_definitions"])
	if err != nil {
		return nil, err
	}

	var authModel []openfga.TypeDefinition
	err = json.Unmarshal(b, &authModel)
	if err != nil {
		return nil, err
	}

	return authModel, nil
}

// CreateAuthModel creates a new authorization model as per the provided type
// definitions and returns its ID. The AuthModelFromJson function can be used
// to convert an authorization model from json to the slice of type definitions
// required by this method.
func (c *Client) CreateAuthModel(ctx context.Context, authModel []openfga.TypeDefinition) (string, error) {
	ar := openfga.NewWriteAuthorizationModelRequest(authModel)
	resp, _, err := c.api.WriteAuthorizationModel(ctx).Body(*ar).Execute()
	if err != nil {
		return "", err
	}
	return resp.GetAuthorizationModelId(), nil
}

// ListAuthModels returns the list of authorization models present on the
// openFGA instance.
func (c *Client) ListAuthModels(ctx context.Context) ([]openfga.AuthorizationModel, error) {
	resp, _, err := c.api.ReadAuthorizationModels(ctx).Execute()
	if err != nil {
		return nil, fmt.Errorf("cannot list authorization models %q", err)
	}
	return resp.GetAuthorizationModels(), nil
}

// GetAuthModel fetches an authorization model by ID from the openFGA instance.
func (c *Client) GetAuthModel(ctx context.Context, ID string) (openfga.AuthorizationModel, error) {
	resp, _, err := c.api.ReadAuthorizationModel(ctx, ID).Execute()
	if err != nil {
		return openfga.AuthorizationModel{}, fmt.Errorf("cannot list authorization models %q", err)
	}
	return resp.GetAuthorizationModel(), nil
}

// GetMatchingTuples fetches all stored relationship tuples that match the given
// input tuple. This method uses the underlying Read API from openFGA. Note that
// this method only fetches actual tuples that were stored in the system. It
// does not show any implied relationships (as defined in the authorization
// model)
//
// This method has some constraints on the types of tuples passed in (the
// constraints are from the underlying openfga.Read API):
//   - Tuple.Target must have the Kind specified. The ID is optional.
//   - If Tuple.Target.ID is not specified then Tuple.Object is mandatory and
//     must be fully specified (Kind & ID & possibly Relation as well).
//   - Alternatively, Tuple can be an empty struct passed in with all nil/empty
//     values. In this case, all tuples from the system are returned.
//
// This method can be used to find all tuples where:
//   - a specific user has a specific relation with objects of a specific type
//     eg: Find all documents where bob is a writer - ("user:bob", "writer", "document:")
//   - a specific user has any relation with objects of a specific type
//     eg: Find all documents related to bob - ("user:bob", "", "document:")
//   - any user has any relation with a specific object
//     eg: Find all documents related by a writer relation - ("", "", "document:planning")
//
// This method is also useful during authorization model migrations.
func (c *Client) GetMatchingTuples(ctx context.Context, tuple Tuple, pageSize int32, paginationToken string) ([]TimestampedTuple, error) {
	rr := openfga.NewReadRequest()
	if pageSize != 0 {
		rr.SetPageSize(pageSize)
	}
	if paginationToken != "" {
		rr.SetContinuationToken(paginationToken)
	}
	if !tuple.isEmpty() {
		if tuple.Target.Kind == "" {
			return nil, errors.New("missing tuple.Target.Kind")
		}
		if tuple.Target.ID == "" && (tuple.Object.Kind == "" || tuple.Object.ID == "") {
			return nil, errors.New("either tuple.Target.ID or tuple.Object must be specified")
		}
		rr.SetTupleKey(tuple.toOpenFGATuple())
	}
	resp, _, err := c.api.Read(ctx).Body(*rr).Execute()
	if err != nil {
		return nil, err
	}
	tuples := make([]TimestampedTuple, len(resp.GetTuples()))
	for _, oTuple := range resp.GetTuples() {
		t, err := fromOpenFGATupleKey(*oTuple.Key)
		if err != nil {
			return nil, fmt.Errorf("could not parse tuple %+v. %w", oTuple, err)
		}
		tuples = append(tuples, TimestampedTuple{
			tuple:     t,
			timestamp: *oTuple.Timestamp,
		})
	}
	return tuples, nil
}
