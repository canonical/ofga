// Copyright 2023 Canonical Ltd.

// Package ofga provides utilities for interacting with an OpenFGA instance.
package ofga

import (
	"context"
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
	ListStores(ctx context.Context) openfga.ApiListStoresRequest
	GetStore(ctx context.Context) openfga.ApiGetStoreRequest
	ReadAuthorizationModel(ctx context.Context, id string) openfga.ApiReadAuthorizationModelRequest
	Write(ctx context.Context) openfga.ApiWriteRequest
	Check(ctx context.Context) openfga.ApiCheckRequest
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

// RemoveRelation removes the specified relation between the objects & targets
// as specified by the given tuples.
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
