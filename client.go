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
	Scheme      string
	Host        string
	Port        string
	Token       string
	StoreID     string
	AuthModelID string
}

// Client is a wrapper over the client provided by OpenFGA
// (https://github.com/openfga/go-sdk). The wrapper contains convenient utility
// methods for interacting with OpenFGA. It also ensures that it is able to
// connect to the specified OpenFGA instance, and verifies the existence of a
// Store and AuthorizationModel if such IDs are provided during configuration.
type Client struct {
	api         openfga.OpenFgaApi
	AuthModelId string
}

// NewClient returns a wrapped OpenFGA API client ensuring all calls are made
// to the provided authorisation model (id) and returns what is necessary.
func NewClient(ctx context.Context, p OpenFGAParams) (*Client, error) {
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
		ApiHost:   fmt.Sprintf("%s:%s", p.Host, p.Port), // The host must be specified without the scheme (i.e. `api.fga.example` instead of `https://api.fga.example`).
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
	client := openfga.NewAPIClient(configuration)
	api := client.OpenFgaApi

	_, response, err := api.ListStores(ctx).Execute()
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		// The response body is only used as extra information in the error
		// message, so if an error occurred while trying to read the response
		// body, we can just ignore it.
		body, _ := io.ReadAll(response.Body)
		return nil, fmt.Errorf("failed to contact the OpenFga server: received %v: %s", response.StatusCode, string(body))
	}

	// If StoreID is present, validate that such a store exists.
	if p.StoreID != "" {
		storeResp, _, err := api.GetStore(ctx).Execute()
		if err != nil {
			zapctx.Error(ctx, fmt.Sprintf("cannot retrieve store %q", err))
			return nil, errors.New("cannot retrieve store")
		}
		zapctx.Info(ctx, "store found", zap.String("storeName", *storeResp.Name))
	}

	// If AuthModelID is present, validate that such an AuthModel exists.
	if p.AuthModelID != "" {
		authModelResp, _, err := api.ReadAuthorizationModel(ctx, p.AuthModelID).Execute()
		if err != nil {
			zapctx.Error(ctx, fmt.Sprintf("cannot retrieve authModel %q", err))
			return nil, errors.New("cannot retrieve authModel")
		}
		zapctx.Info(ctx, "auth model found", zap.String("authModelID", authModelResp.AuthorizationModel.GetId()))
	}
	return &Client{
		api:         client.OpenFgaApi,
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
	zapctx.Debug(ctx, "check request internal resp code", zap.Int("code", httpResp.StatusCode))
	allowed := checkResp.GetAllowed()
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
