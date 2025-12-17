// Copyright 2023 Canonical Ltd.
// Licensed under the LGPL license, see LICENSE file for details.

// Package ofga provides utilities for interacting with an OpenFGA instance.
package ofga

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/juju/zaputil/zapctx"
	openfga "github.com/openfga/go-sdk"
	"github.com/openfga/go-sdk/credentials"
	"github.com/openfga/go-sdk/telemetry"
	"go.uber.org/zap"
)

const ignoreMissingOnDelete = "ignore"
const ignoreDuplicateOnWrite = "ignore"

type writeOption func(wr *openfga.WriteRequestWrites) error
type deleteOption func(dr *openfga.WriteRequestDeletes) error

// OpenFGAParams holds parameters needed to connect to the OpenFGA server.
type OpenFGAParams struct {
	// Scheme must be `http` or `https`.
	Scheme string
	// Host is the URL to the OpenFGA server and must be specified without the
	// scheme (i.e. `api.fga.example` instead of `https://api.fga.example`)
	Host string
	// Port specifies the port on which the server is running.
	Port string
	// Token specifies the authentication token to use while communicating with
	// the server.
	Token string
	// StoreID specifies the ID of the OpenFGA Store to be used for
	// authorization checks.
	StoreID string
	// AuthModelID specifies the ID of the OpenFGA Authorization model to be
	// used for authorization checks.
	AuthModelID string
	// Telemetry specifies the OpenTelemetry metrics configuration.
	Telemetry *telemetry.Configuration
	// HTTPClient optionally specifies http.Client to allow
	// for advanced customizations.
	HTTPClient *http.Client
}

// OpenFgaApi defines the methods of the underlying api client that our Client
// depends upon.
type OpenFgaApi interface {
	Check(ctx context.Context, storeID string) openfga.ApiCheckRequest
	CreateStore(ctx context.Context) openfga.ApiCreateStoreRequest
	Expand(ctx context.Context, storeID string) openfga.ApiExpandRequest
	GetStore(ctx context.Context, storeID string) openfga.ApiGetStoreRequest
	ListObjects(ctx context.Context, storeID string) openfga.ApiListObjectsRequest
	ListStores(ctx context.Context) openfga.ApiListStoresRequest
	Read(ctx context.Context, storeID string) openfga.ApiReadRequest
	ReadAuthorizationModel(ctx context.Context, storeID string, id string) openfga.ApiReadAuthorizationModelRequest
	ReadAuthorizationModels(ctx context.Context, storeID string) openfga.ApiReadAuthorizationModelsRequest
	ReadChanges(ctx context.Context, storeID string) openfga.ApiReadChangesRequest
	Write(ctx context.Context, storeID string) openfga.ApiWriteRequest
	WriteAuthorizationModel(ctx context.Context, storeID string) openfga.ApiWriteAuthorizationModelRequest
}

// Client is a wrapper over the client provided by OpenFGA
// (https://github.com/openfga/go-sdk). The wrapper contains convenient utility
// methods for interacting with OpenFGA. It also ensures that it is able to
// connect to the specified OpenFGA instance, and verifies the existence of a
// Store and AuthorizationModel if such IDs are provided during configuration.
type Client struct {
	api         OpenFgaApi
	authModelID string
	storeID     string
}

// NewClient returns a wrapped OpenFGA API client ensuring all calls are made
// to the provided authorisation model (id) and returns what is necessary.
func NewClient(ctx context.Context, p OpenFGAParams) (*Client, error) {
	if p.Host == "" {
		return nil, errors.New("invalid OpenFGA configuration: missing host")
	}
	if p.Port == "" {
		return nil, errors.New("invalid OpenFGA configuration: missing port")
	}
	if p.StoreID == "" && p.AuthModelID != "" {
		return nil, errors.New("invalid OpenFGA configuration: AuthModelID specified without a StoreID")
	}
	zapctx.Info(ctx, "configuring OpenFGA client",
		zap.String("scheme", p.Scheme),
		zap.String("host", p.Host),
		zap.String("port", p.Port),
		zap.String("store", p.StoreID),
	)

	config := openfga.Configuration{
		ApiUrl: fmt.Sprintf("%s://%s:%s", p.Scheme, p.Host, p.Port),
	}
	if p.Token != "" {
		config.Credentials = &credentials.Credentials{
			Method: credentials.CredentialsMethodApiToken,
			Config: &credentials.Config{
				ApiToken: p.Token,
			},
		}
	} else {
		config.Credentials = &credentials.Credentials{
			Method: credentials.CredentialsMethodNone,
		}
	}
	if p.HTTPClient != nil {
		config.HTTPClient = p.HTTPClient
		// When a custom HTTPClient is provided in OpenFGA configuration,
		// it does not add authorization headers, so we manually add them here.
		_, headers := config.Credentials.GetHttpClientAndHeaderOverrides(config.GetRetryParams(), config.Debug)
		defaultHeaders := make(map[string]string)
		if len(headers) != 0 {
			for idx := range headers {
				defaultHeaders[headers[idx].Key] = headers[idx].Value
			}
		}
		config.DefaultHeaders = defaultHeaders
	}
	if p.Telemetry != nil {
		config.Telemetry = p.Telemetry
	}
	configuration, err := openfga.NewConfiguration(config)
	if err != nil {
		return nil, fmt.Errorf("invalid OpenFGA configuration: %v", err)
	}
	client := openfga.NewAPIClient(configuration)
	api := client.OpenFgaApi

	_, _, err = api.ListStores(ctx).Execute()
	if err != nil {
		zapctx.Error(ctx, fmt.Sprintf("cannot list stores: %v", err))
		return nil, fmt.Errorf("cannot list stores: %v", err)
	}

	// If StoreID is present, validate that such a store exists.
	if p.StoreID != "" {
		storeResp, _, err := api.GetStore(ctx, p.StoreID).Execute()
		if err != nil {
			zapctx.Error(ctx, fmt.Sprintf("cannot retrieve store: %v", err))
			return nil, fmt.Errorf("cannot retrieve store: %v", err)
		}
		zapctx.Info(ctx, "store found", zap.String("storeName", storeResp.GetName()))
	}

	// If AuthModelID is present, validate that such an AuthModel exists.
	if p.AuthModelID != "" {
		authModelResp, _, err := api.ReadAuthorizationModel(ctx, p.StoreID, p.AuthModelID).Execute()
		if err != nil {
			zapctx.Error(ctx, fmt.Sprintf("cannot retrieve authModel: %v", err))
			return nil, fmt.Errorf("cannot retrieve authModel: %v", err)
		}
		zapctx.Info(ctx, "auth model found", zap.String("authModelID", authModelResp.AuthorizationModel.GetId()))
	}
	return &Client{
		api:         api,
		authModelID: p.AuthModelID,
		storeID:     p.StoreID,
	}, nil
}

// AuthModelID returns the currently configured authorization model ID.
func (c *Client) AuthModelID() string {
	return c.authModelID
}

// SetAuthModelID sets the authorization model ID to be used by the client.
func (c *Client) SetAuthModelID(authModelID string) {
	c.authModelID = authModelID
}

// StoreID gets the currently configured store ID.
func (c *Client) StoreID() string {
	return c.storeID
}

// SetStoreID sets the store ID to be used by the client.
func (c *Client) SetStoreID(storeID string) {
	c.storeID = storeID
}

// AddRelation adds the specified relation(s) between the objects & targets as
// specified by the given tuple(s).
func (c *Client) AddRelation(ctx context.Context, tuples ...Tuple) error {
	return c.AddRemoveRelations(ctx, tuples, nil)
}

// AddRelationIdempotent adds the specified relation(s) between the objects & targets as
// specified by the given tuple(s), and ignores duplicate tuples that already exist in the store.
// Note: Duplicates within the same request are not allowed and will cause an error.
// It requires OpenFGA server version >= 1.10.0.
func (c *Client) AddRelationIdempotent(ctx context.Context, tuples ...Tuple) error {
	return c.AddRemoveRelationsIdempotent(ctx, tuples, nil)
}

// CheckRelation checks whether the specified relation exists (either directly
// or indirectly) between the object and the target specified by the tuple.
//
// Additionally, this method allows specifying contextualTuples to augment the
// check request with temporary, non-persistent relationship tuples that exist
// solely within the scope of this specific check. Contextual tuples are not
// written to the store but are taken into account for this particular check
// request as if they were present in the store.
func (c *Client) CheckRelation(ctx context.Context, tuple Tuple, contextualTuples ...Tuple) (bool, error) {
	return c.checkRelation(ctx, tuple, false, contextualTuples...)
}

// CheckRelationWithTracing verifies that the specified relation exists (either
// directly or indirectly) between the object and the target as specified by
// the tuple. This method enables the tracing option.
//
// Additionally, this method allows specifying contextualTuples to augment the
// check request with temporary, non-persistent relationship tuples that exist
// solely within the scope of this specific check. Contextual tuples are not
// written to the store but are taken into account for this particular check
// request as if they were present in the store.
func (c *Client) CheckRelationWithTracing(ctx context.Context, tuple Tuple, contextualTuples ...Tuple) (bool, error) {
	return c.checkRelation(ctx, tuple, true, contextualTuples...)
}

// checkRelation internal implementation for check relation procedure.
func (c *Client) checkRelation(ctx context.Context, tuple Tuple, trace bool, contextualTuples ...Tuple) (bool, error) {
	zapctx.Debug(
		ctx,
		"check request internal",
		zap.String("tuple object", tuple.Object.String()),
		zap.String("tuple relation", tuple.Relation.String()),
		zap.String("tuple target object", tuple.Target.String()),
		zap.Bool("trace", trace),
		zap.Int("contextual tuples", len(contextualTuples)),
	)
	cr := openfga.NewCheckRequest(*tuple.ToOpenFGACheckRequestTupleKey())
	cr.SetAuthorizationModelId(c.authModelID)

	if len(contextualTuples) > 0 {
		keys := tuplesToOpenFGATupleKeys(contextualTuples)
		cr.SetContextualTuples(*openfga.NewContextualTupleKeys(keys))
	}

	cr.SetTrace(trace)

	checkResp, httpResp, err := c.api.Check(ctx, c.storeID).Body(*cr).Execute()
	if err != nil {
		zapctx.Error(ctx, fmt.Sprintf("cannot execute Check request: %v", err))
		return false, fmt.Errorf("cannot check relation: %v", err)
	}
	allowed := checkResp.GetAllowed()
	zapctx.Debug(ctx, "check request internal resp code", zap.Int("code", httpResp.StatusCode), zap.Bool("allowed", allowed))
	return allowed, nil
}

// RemoveRelation removes the specified relation(s) between the objects &
// targets as specified by the given tuples.
func (c *Client) RemoveRelation(ctx context.Context, tuples ...Tuple) error {
	return c.AddRemoveRelations(ctx, nil, tuples)
}

// RemoveRelationIdempotent removes the specified relation(s) between the objects &
// targets as specified by the given tuples and ignores missing tuples that don't exist in the store.
// Note: Duplicates within the same request are not allowed and will cause an error.
// It requires OpenFGA server version >= 1.10.0.
func (c *Client) RemoveRelationIdempotent(ctx context.Context, tuples ...Tuple) error {
	return c.AddRemoveRelationsIdempotent(ctx, nil, tuples)
}

// AddRemoveRelations adds and removes the specified relation tuples in a single
// atomic write operation. If you want to solely add relations or solely remove
// relations, consider using the AddRelation or RemoveRelation methods instead.
func (c *Client) AddRemoveRelations(ctx context.Context, addTuples, removeTuples []Tuple) error {
	return c.addRemoveRelations(ctx, addTuples, removeTuples, nil, nil)
}

// AddRemoveRelationsIdempotent adds and removes the specified relation tuples in a single
// atomic write operation. If you want to solely add relations or solely remove
// relations, consider using the AddRelation or RemoveRelation methods instead.
// This method ignores missing tuples during removal and duplicate tuples during addition that already exist in the store.
// Note: Duplicates within the same request are not allowed and will cause an error.
// It requires OpenFGA server version >= 1.10.0.
func (c *Client) AddRemoveRelationsIdempotent(ctx context.Context, addTuples, removeTuples []Tuple) error {
	return c.addRemoveRelations(ctx, addTuples, removeTuples, []writeOption{
		func(wr *openfga.WriteRequestWrites) error {
			wr.SetOnDuplicate(ignoreDuplicateOnWrite)
			return nil
		},
	}, []deleteOption{
		func(dr *openfga.WriteRequestDeletes) error {
			dr.SetOnMissing(ignoreMissingOnDelete)
			return nil
		},
	})
}

func (c *Client) addRemoveRelations(ctx context.Context, addTuples, removeTuples []Tuple, requestWrites []writeOption, requestDeletes []deleteOption) error {
	wr := openfga.NewWriteRequest()
	wr.SetAuthorizationModelId(c.authModelID)

	if len(addTuples) > 0 {
		addTupleKeys := tuplesToOpenFGATupleKeys(addTuples)
		wReq := openfga.NewWriteRequestWrites(addTupleKeys)
		for _, opt := range requestWrites {
			if err := opt(wReq); err != nil {
				return err
			}
		}
		wr.SetWrites(*wReq)
	}
	if len(removeTuples) > 0 {
		removeTupleKeys := tuplesToOpenFGATupleKeysWithoutCondition(removeTuples)
		delReq := openfga.NewWriteRequestDeletes(removeTupleKeys)
		for _, opt := range requestDeletes {
			if err := opt(delReq); err != nil {
				return err
			}
		}
		wr.SetDeletes(*delReq)
	}
	_, _, err := c.api.Write(ctx, c.storeID).Body(*wr).Execute()
	if err != nil {
		zapctx.Error(ctx, fmt.Sprintf("cannot execute Write request: %v", err))
		return fmt.Errorf("cannot add or remove relations: %v", err)
	}
	return nil
}

// CreateStore creates a new store on the openFGA instance and returns its ID.
func (c *Client) CreateStore(ctx context.Context, name string) (string, error) {
	csr := openfga.NewCreateStoreRequest(name)
	resp, _, err := c.api.CreateStore(ctx).Body(*csr).Execute()
	if err != nil {
		zapctx.Error(ctx, fmt.Sprintf("cannot execute CreateStore request: %v", err))
		return "", fmt.Errorf("cannot create store: %v", err)
	}
	return resp.GetId(), nil
}

// ListStores returns the list of stores present on the openFGA instance. If
// pageSize is set to 0, then the default pageSize is used. If this is the
// initial request, an empty string should be passed in as the
// continuationToken.
func (c *Client) ListStores(ctx context.Context, pageSize int32, continuationToken string) (openfga.ListStoresResponse, error) {
	lsr := c.api.ListStores(ctx)

	if pageSize != 0 {
		lsr = lsr.PageSize(pageSize)
	}
	if continuationToken != "" {
		lsr = lsr.ContinuationToken(continuationToken)
	}

	resp, _, err := lsr.Execute()
	if err != nil {
		zapctx.Error(ctx, fmt.Sprintf("cannot execute ListStores request: %v", err))
		return openfga.ListStoresResponse{}, fmt.Errorf("cannot list stores: %v", err)
	}
	return resp, nil
}

// ReadChanges returns a paginated list of tuple changes (additions and
// deletions) sorted by ascending time. The response will include a continuation
// token that can be used to get the next set of changes. If there are no
// changes after the provided continuation token, the same token will be
// returned in order for it to be used when new changes are recorded. If no
// tuples have been added or removed, this token will be empty. The entityType
// parameter can be used to restrict the response to show only changes affecting
// a specific type. For more information, check: https://openfga.dev/docs/interacting/read-tuple-changes#02-get-changes-for-all-object-types
func (c *Client) ReadChanges(ctx context.Context, entityType string, pageSize int32, continuationToken string) (openfga.ReadChangesResponse, error) {
	rcr := c.api.ReadChanges(ctx, c.storeID)
	rcr = rcr.Type_(entityType)
	if pageSize != 0 {
		rcr = rcr.PageSize(pageSize)
	}
	if continuationToken != "" {
		rcr = rcr.ContinuationToken(continuationToken)
	}

	resp, _, err := rcr.Execute()
	if err != nil {
		zapctx.Error(ctx, fmt.Sprintf("cannot execute ReadChanges request: %v", err))
		return openfga.ReadChangesResponse{}, fmt.Errorf("cannot read changes: %v", err)
	}
	return resp, nil
}

// AuthModelFromJSON converts the input json representation of an authorization
// model into an [openfga.AuthorizationModel] that can be used with the API.
func AuthModelFromJSON(data []byte) (*openfga.AuthorizationModel, error) {
	var parsed openfga.AuthorizationModel
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("cannot unmarshal JSON auth model: %v", err)
	}

	if parsed.TypeDefinitions == nil {
		return nil, fmt.Errorf(`"type_definitions" field not found`)
	}

	return &parsed, nil
}

// CreateAuthModel creates a new authorization model as per the provided type
// definitions and schemaVersion and returns its ID. The [AuthModelFromJSON]
// function can be used to convert an authorization model from json to the
// slice of type definitions required by this method.
func (c *Client) CreateAuthModel(ctx context.Context, authModel *openfga.AuthorizationModel) (string, error) {
	ar := openfga.NewWriteAuthorizationModelRequest(authModel.TypeDefinitions, authModel.SchemaVersion)
	ar.SetSchemaVersion(authModel.SchemaVersion)
	resp, _, err := c.api.WriteAuthorizationModel(ctx, c.storeID).Body(*ar).Execute()
	if err != nil {
		zapctx.Error(ctx, fmt.Sprintf("cannot execute WriteAuthorizationModel request: %v", err))
		return "", fmt.Errorf("cannot create auth model: %v", err)
	}
	return resp.GetAuthorizationModelId(), nil
}

// ListAuthModels returns the list of authorization models present on the
// openFGA instance. If pageSize is set to 0, then the default pageSize is
// used. If this is the initial request, an empty string should be passed in
// as the continuationToken.
func (c *Client) ListAuthModels(ctx context.Context, pageSize int32, continuationToken string) (openfga.ReadAuthorizationModelsResponse, error) {
	rar := c.api.ReadAuthorizationModels(ctx, c.storeID)
	if pageSize != 0 {
		rar = rar.PageSize(pageSize)
	}
	if continuationToken != "" {
		rar = rar.ContinuationToken(continuationToken)
	}
	resp, _, err := rar.Execute()
	if err != nil {
		zapctx.Error(ctx, fmt.Sprintf("cannot execute ReadAuthorizationModels request: %v", err))
		return openfga.ReadAuthorizationModelsResponse{}, fmt.Errorf("cannot list authorization models: %v", err)
	}
	return resp, nil
}

// GetAuthModel fetches an authorization model by ID from the openFGA instance.
func (c *Client) GetAuthModel(ctx context.Context, ID string) (openfga.AuthorizationModel, error) {
	resp, _, err := c.api.ReadAuthorizationModel(ctx, c.storeID, ID).Execute()
	if err != nil {
		zapctx.Error(ctx, fmt.Sprintf("cannot execute ReadAuthorizationModel request: %v", err))
		return openfga.AuthorizationModel{}, fmt.Errorf("cannot list authorization models: %v", err)
	}
	return resp.GetAuthorizationModel(), nil
}

// validateTupleForFindMatchingTuples validates that the input tuples to the
// FindMatchingTuples method complies with the API requirements.
func validateTupleForFindMatchingTuples(tuple Tuple) error {
	if tuple.Target.Kind == "" {
		return errors.New("missing tuple.Target.Kind")
	}
	if tuple.Target.ID == "" && (tuple.Object.Kind == "" || tuple.Object.ID == "") {
		return errors.New("either tuple.Target.ID or tuple.Object must be specified")
	}
	if tuple.Target.Relation != "" {
		return errors.New("tuple.Target.Relation must not be set")
	}
	return nil
}

// FindMatchingTuples fetches all stored relationship tuples that match the
// given input tuple. This method uses the underlying Read API from openFGA.
// Note that this method only fetches actual tuples that were stored in the
// system. It does not show any implied relationships (as defined in the
// authorization model)
//
// This method has some constraints on the tuples passed in (the
// constraints are from the underlying openfga.Read API):
//   - Tuple.Target must have the Kind specified. The ID is optional.
//   - If Tuple.Target.ID is not specified then Tuple.Object is mandatory and
//     must be fully specified (Kind & ID & possibly Relation as well).
//   - Alternatively, Tuple can be an empty struct passed in with all nil/empty
//     values. In this case, all tuples from the system are returned.
//
// This method can be used to find all tuples where:
//   - a specific user has a specific relation with objects of a specific type
//     eg: Find all documents where bob is a writer -
//     ("user:bob", "writer", "document:")
//   - a specific user has any relation with objects of a specific type
//     eg: Find all documents related to bob - ("user:bob", "", "document:")
//   - any user has any relation with a specific object
//     eg: Find all documents related by a writer relation -
//     ("", "", "document:planning")
//
// This method is also useful during authorization model migrations.
func (c *Client) FindMatchingTuples(ctx context.Context, tuple Tuple, pageSize int32, continuationToken string) ([]TimestampedTuple, string, error) {
	rr := openfga.NewReadRequest()
	if !tuple.isEmpty() {
		if err := validateTupleForFindMatchingTuples(tuple); err != nil {
			return nil, "", fmt.Errorf("invalid tuple for FindMatchingTuples: %v", err)
		}
		rr.SetTupleKey(*tuple.ToOpenFGAReadRequestTupleKey())
	}
	if pageSize != 0 {
		rr.SetPageSize(pageSize)
	}
	if continuationToken != "" {
		rr.SetContinuationToken(continuationToken)
	}
	resp, _, err := c.api.Read(ctx, c.storeID).Body(*rr).Execute()
	if err != nil {
		zapctx.Error(ctx, fmt.Sprintf("cannot execute Read request: %v", err))
		return nil, "", fmt.Errorf("cannot fetch matching tuples: %v", err)
	}
	tuples := make([]TimestampedTuple, 0, len(resp.GetTuples()))
	for _, oTuple := range resp.GetTuples() {
		t, err := FromOpenFGATupleKey(oTuple.Key)
		if err != nil {
			zapctx.Error(ctx, fmt.Sprintf("cannot parse tuple from Read response: %v", err))
			return nil, "", fmt.Errorf("cannot parse tuple %+v, %v", oTuple, err)
		}
		tuples = append(tuples, TimestampedTuple{
			Tuple:     t,
			Timestamp: oTuple.Timestamp,
		})
	}
	return tuples, resp.GetContinuationToken(), nil
}

// FindUsersByRelation fetches the list of users that have a specific
// relation with a specific target object. This method not only searches
// through the relationship tuples present in the system, but also takes into
// consideration the authorization model and the relationship tuples implied
// by the model (for instance, a writer of a document is also a viewer of
// the document), and recursively expands these relationships upto `maxDepth`
// levels deep to obtain the final list of users. A `maxDepth` of `1` causes
// the current tuple to be expanded and the immediate expansion results to be
// returned. `maxDepth` can be any positive number.
//
// This method requires that Tuple.Target and Tuple.Relation be specified.
//
// Note that this method call is expensive and has high latency, and should be
// used with caution. The official docs state that the underlying API method
// was intended to be used for debugging: https://openfga.dev/docs/interacting/relationship-queries#caveats-and-when-not-to-use-it-2
func (c *Client) FindUsersByRelation(ctx context.Context, tuple Tuple, maxDepth int) ([]Entity, error) {
	if maxDepth < 1 {
		return nil, errors.New(`maxDepth must be greater than or equal to 1`)
	}
	userStrings, err := c.findUsersByRelation(ctx, tuple, maxDepth)
	if err != nil {
		return nil, err
	}
	var users []Entity
	for u := range userStrings {
		user, err := ParseEntity(u)
		if err != nil {
			return nil, fmt.Errorf("cannot parse entity %v from Expand response: %v", u, err)
		}
		users = append(users, user)
	}
	return users, nil
}

// validateTupleForFindUsersByRelation validates that the input tuples to the
// FindMatchingTuples method complies with the API requirements.
func validateTupleForFindUsersByRelation(tuple Tuple) error {
	if tuple.Target.Kind == "" || tuple.Target.ID == "" {
		return errors.New("missing tuple.Target")
	}
	if tuple.Target.Relation != "" {
		return errors.New("tuple.Target.Relation must not be set")
	}
	if tuple.Relation == "" {
		return errors.New("missing tuple.Relation")
	}
	return nil
}

// findUsersByRelation is the internal implementation for
// FindUsersByRelation. It returns a set of userStrings representing the
// list of users that have access to the specified object via the specified
// relation.
func (c *Client) findUsersByRelation(ctx context.Context, tuple Tuple, maxDepth int) (map[string]bool, error) {
	if err := validateTupleForFindUsersByRelation(tuple); err != nil {
		return nil, fmt.Errorf("invalid tuple for FindUsersByRelation: %v", err)
	}
	// If we have reached the maxDepth and shouldn't expand the results further,
	// return the current userSet.
	if maxDepth == 0 {
		userSet := tuple.Target
		userSet.Relation = tuple.Relation
		return map[string]bool{
			userSet.String(): true,
		}, nil
	}

	er := openfga.NewExpandRequest(*tuple.ToOpenFGAExpandRequestTupleKey())
	er.SetAuthorizationModelId(c.authModelID)
	resp, _, err := c.api.Expand(ctx, c.storeID).Body(*er).Execute()
	if err != nil {
		zapctx.Error(ctx, fmt.Sprintf("cannot execute Expand request: %v", err))
		return nil, fmt.Errorf("cannot execute Expand request: %v", err)
	}

	tree := resp.GetTree()
	if !tree.HasRoot() {
		return nil, errors.New("tree from Expand response has no root")
	}
	root := tree.GetRoot()
	leaves, err := c.traverseTree(ctx, &root, maxDepth-1)
	if err != nil {
		return nil, fmt.Errorf("cannot expand the intermediate results: %v", err)
	}
	return leaves, nil
}

// traverseTree will recursively expand the tree returned by an openfga Expand
// call to find all users that have the specified relation to the specified
// target entity.
func (c *Client) traverseTree(ctx context.Context, node *openfga.Node, maxDepth int) (map[string]bool, error) {
	logError := func(message, nodeType string, n interface{}) {
		data, _ := json.Marshal(n)
		zapctx.Error(ctx, message, zap.String(nodeType, string(data)))
	}

	// If this is a union node, we traverse all child nodes recursively to get
	// the leaf nodes and return the aggregated results.
	if node.HasUnion() {
		union := node.GetUnion()
		users := make(map[string]bool)
		for _, childNode := range union.GetNodes() {
			childNode := childNode
			childNodeUsers, err := c.traverseTree(ctx, &childNode, maxDepth)
			if err != nil {
				return nil, err
			}
			for userString := range childNodeUsers {
				users[userString] = true
			}
		}
		return users, nil
	}

	if !node.HasLeaf() {
		logError("unknown node type", "node", node)
		return nil, errors.New("unknown node type")
	}

	leaf := node.GetLeaf()
	// A leaf node may contain either:
	// - users: these are the users/userSets that have the specified
	//		relation with the specified object via relationship tuples that
	//		were added to the system.
	// - computed userSets: these are the userSets that have the specified
	//		relation with the specified object via an implied relationship
	//		defined by the authorization model. Example: All writers of a
	//		document are viewers of the document.
	// - tupleToUserSet: these are userSets that have the specified
	//		relation with the specified object via an indirect implied
	//		relation through other types defined in the authorization
	//		model. Example: Any user that is assigned the editor relation
	//		on a folder is automatically assigned the editor relation to
	//		any documents that belong to that folder.
	//
	// If the leaf node contains computedSets or tupleToUserSets, we need
	// to expand them further to obtain individual users.
	if leaf.HasUsers() {
		users, err := c.expand(ctx, maxDepth, leaf.Users.Users...)
		if err != nil {
			return nil, err
		}
		return users, nil
	}

	if leaf.HasComputed() {
		return c.expandComputed(ctx, maxDepth, leaf, leaf.GetComputed())
	}

	if leaf.HasTupleToUserset() {
		tupleToUserSet := leaf.GetTupleToUserset()
		computed := tupleToUserSet.GetComputed()
		if len(computed) > 0 {
			return c.expandComputed(ctx, maxDepth, leaf, computed...)
		}
	}

	logError("unknown leaf type", "leaf", leaf)
	return nil, errors.New("unknown leaf type")
}

// expandComputed is a helper method to expand a computedSet into its
// constituent users. The leaf parameter of this function is used for
// logging purposes only.
func (c *Client) expandComputed(ctx context.Context, maxDepth int, leaf openfga.Leaf, computedList ...openfga.Computed) (map[string]bool, error) {
	logError := func(message, nodeType string, n interface{}) {
		data, _ := json.Marshal(n)
		zapctx.Error(ctx, message, zap.String(nodeType, string(data)))
	}
	users := make(map[string]bool)
	for _, computed := range computedList {
		userSet := computed.GetUserset()
		if userSet != "" {
			found, err := c.expand(ctx, maxDepth, userSet)
			if err != nil {
				return nil, err
			}
			for userString := range found {
				users[userString] = true
			}
		} else {
			logError("missing userSet", "leaf", leaf)
			return nil, errors.New("missing userSet")
		}
	}
	return users, nil
}

// expand checks all userStrings in the input list and expands any userSets
// that are present into the constituent individual users. Example:
// "team:planning#members" would be expanded into "user:abc", "user:xyz", etc.
func (c *Client) expand(ctx context.Context, maxDepth int, userStrings ...string) (map[string]bool, error) {
	users := make(map[string]bool, len(userStrings))
	for _, u := range userStrings {
		tokens := strings.Split(u, "#")
		switch len(tokens) {
		case 1:
			// No '#' is present so this is an individual user. Add it to the
			// map and continue.
			users[u] = true
		case 2:
			// We need to expand this userSet to obtain the individual
			// users that it contains.
			t := openfga.NewTupleKeyWithDefaults()
			t.SetRelation(tokens[1])
			t.SetObject(tokens[0])
			tuple, err := FromOpenFGATupleKey(*t)
			if err != nil {
				return nil, fmt.Errorf("failed to parse tuple %s, %v", u, err)
			}
			found, err := c.findUsersByRelation(ctx, tuple, maxDepth)
			if err != nil {
				return nil, fmt.Errorf("failed to expand %s, %v", u, err)
			}
			for userString := range found {
				users[userString] = true
			}
		default:
			zapctx.Error(ctx, fmt.Sprintf("unknown user representation: %s", u))
			return nil, fmt.Errorf("unknown user representation: %s", u)
		}
	}
	return users, nil
}

// validateTupleForFindAccessibleObjectsByRelation validates that the input
// tuples to the FindAccessibleObjectsByRelation method complies with the API
// requirements.
func validateTupleForFindAccessibleObjectsByRelation(tuple Tuple) error {
	if tuple.Object.Kind == "" || tuple.Object.ID == "" {
		return errors.New("missing tuple.Object")
	}
	if tuple.Relation == "" {
		return errors.New("missing tuple.Relation")
	}
	if tuple.Target.Kind == "" || tuple.Target.Relation != "" || tuple.Target.ID != "" {
		return errors.New("only tuple.Target.Kind must be set")
	}
	return nil
}

// FindAccessibleObjectsByRelation returns a list of all objects of a specified
// type that a user (or any other entity) has access to via the specified
// relation. This method checks both actual tuples and implied relations by the
// authorization model. This method does not recursively expand relations,
// it will simply check for exact matches between the specified user/entity
// and the target entity.
//
// This method has some constraints on the tuples passed in (the
// constraints are from the underlying openfga.ListObjects API):
//   - The tuple.Object field must have only the Kind and ID fields set.
//   - The tuple.Relation field must be set.
//   - The tuple.Target field must specify only the Kind.
//
// Note that there are some important caveats to using this method (suboptimal
// performance depending on the authorization model, experimental, subject to
// context deadlines, See: https://openfga.dev/docs/interacting/relationship-queries#caveats-and-when-not-to-use-it-3
func (c *Client) FindAccessibleObjectsByRelation(ctx context.Context, tuple Tuple, contextualTuples ...Tuple) ([]Entity, error) {
	if err := validateTupleForFindAccessibleObjectsByRelation(tuple); err != nil {
		return nil, fmt.Errorf("invalid tuple for FindAccessibleObjectsByRelation: %v", err)
	}

	lor := openfga.NewListObjectsRequestWithDefaults()
	lor.SetAuthorizationModelId(c.authModelID)
	lor.SetUser(tuple.Object.String())
	lor.SetRelation(tuple.Relation.String())
	lor.SetType(tuple.Target.Kind.String())

	if len(contextualTuples) > 0 {
		keys := tuplesToOpenFGATupleKeys(contextualTuples)
		lor.SetContextualTuples(*openfga.NewContextualTupleKeys(keys))
	}

	resp, _, err := c.api.ListObjects(ctx, c.storeID).Body(*lor).Execute()
	if err != nil {
		zapctx.Error(ctx, fmt.Sprintf("cannot execute ListObjects request: %v", err))
		return nil, fmt.Errorf("cannot list objects: %v", err)
	}

	objects := make([]Entity, 0, len(resp.GetObjects()))
	for _, o := range resp.GetObjects() {
		e, err := ParseEntity(o)
		if err != nil {
			return nil, fmt.Errorf("cannot parse entity %s from ListObjects response: %v", o, err)
		}
		objects = append(objects, e)
	}

	return objects, nil
}
