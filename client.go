// Copyright 2023 Canonical Ltd.

// Package ofga provides utilities for interacting with an OpenFGA instance.
package ofga

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/juju/zaputil/zapctx"
	openfga "github.com/openfga/go-sdk"
	"github.com/openfga/go-sdk/credentials"
	"go.uber.org/zap"
)

// OpenFGAParams holds parameters needed to connect to the OpenFGA server.
type OpenFGAParams struct {
	Scheme string
	// Host must be specified without the scheme
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
	Expand(ctx context.Context) openfga.ApiExpandRequest
	GetStore(ctx context.Context) openfga.ApiGetStoreRequest
	ListObjects(ctx context.Context) openfga.ApiListObjectsRequest
	ListStores(ctx context.Context) openfga.ApiListStoresRequest
	Read(ctx context.Context) openfga.ApiReadRequest
	ReadAuthorizationModel(ctx context.Context, id string) openfga.ApiReadAuthorizationModelRequest
	ReadAuthorizationModels(ctx context.Context) openfga.ApiReadAuthorizationModelsRequest
	ReadChanges(ctx context.Context) openfga.ApiReadChangesRequest
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

// jsonAuthModel represents the structure of an authorization model contained
// in a json string.
type jsonAuthModel struct {
	TypeDefinitions []openfga.TypeDefinition `json:"type_definitions"`
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
		storeResp, _, err := api.GetStore(ctx).Execute()
		if err != nil {
			zapctx.Error(ctx, fmt.Sprintf("cannot retrieve store: %v", err))
			return nil, fmt.Errorf("cannot retrieve store: %v", err)
		}
		zapctx.Info(ctx, "store found", zap.String("storeName", storeResp.GetName()))
	}

	// If AuthModelID is present, validate that such an AuthModel exists.
	if p.AuthModelID != "" {
		authModelResp, _, err := api.ReadAuthorizationModel(ctx, p.AuthModelID).Execute()
		if err != nil {
			zapctx.Error(ctx, fmt.Sprintf("cannot retrieve authModel: %v", err))
			return nil, fmt.Errorf("cannot retrieve authModel: %v", err)
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
		zapctx.Error(ctx, fmt.Sprintf("cannot execute Write request: %v", err))
		return fmt.Errorf("cannot add relation: %v", err)
	}
	return nil
}

// CheckRelation verifies that the specified relation exists (either directly or
// indirectly) between the object and the target as specified by the tuple.
func (c *Client) CheckRelation(ctx context.Context, tuple Tuple, contextualTuples ...Tuple) (bool, error) {
	return c.checkRelation(ctx, tuple, false, contextualTuples...)
}

// CheckRelation verifies that the specified relation exists (either directly or
// indirectly) between the object and the target as specified by the tuple. This
// also enables the tracing option.
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
	cr := openfga.NewCheckRequest(tuple.toOpenFGATuple())
	cr.SetAuthorizationModelId(c.AuthModelId)

	if len(contextualTuples) > 0 {
		keys := make([]openfga.TupleKey, 0, len(contextualTuples))

		for _, ct := range contextualTuples {
			keys = append(keys, ct.toOpenFGATuple())
		}

		cr.SetContextualTuples(*openfga.NewContextualTupleKeys(keys))
	}

	if trace {
		cr.SetTrace(true)
	}

	checkResp, httpResp, err := c.api.Check(ctx).Body(*cr).Execute()
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
		zapctx.Error(ctx, fmt.Sprintf("cannot execute Write request: %v", err))
		return fmt.Errorf("cannot remove relation: %v", err)
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
	rcr := c.api.ReadChanges(ctx)
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
// model into a slice of TypeDefinitions that can be used with the API.
func AuthModelFromJSON(data []byte) ([]openfga.TypeDefinition, error) {
	var parsed jsonAuthModel
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("cannot unmarshal JSON auth model: %v", err)
	}

	if parsed.TypeDefinitions == nil {
		return nil, fmt.Errorf(`"type_definitions" field not found`)
	}

	return parsed.TypeDefinitions, nil
}

// CreateAuthModel creates a new authorization model as per the provided type
// definitions and returns its ID. The AuthModelFromJSON function can be used
// to convert an authorization model from json to the slice of type definitions
// required by this method.
func (c *Client) CreateAuthModel(ctx context.Context, authModel []openfga.TypeDefinition) (string, error) {
	ar := openfga.NewWriteAuthorizationModelRequest(authModel)
	resp, _, err := c.api.WriteAuthorizationModel(ctx).Body(*ar).Execute()
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
	rar := c.api.ReadAuthorizationModels(ctx)
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
	resp, _, err := c.api.ReadAuthorizationModel(ctx, ID).Execute()
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
		rr.SetTupleKey(tuple.toOpenFGATuple())
	}
	if pageSize != 0 {
		rr.SetPageSize(pageSize)
	}
	if continuationToken != "" {
		rr.SetContinuationToken(continuationToken)
	}
	resp, _, err := c.api.Read(ctx).Body(*rr).Execute()
	if err != nil {
		zapctx.Error(ctx, fmt.Sprintf("cannot execute Read request: %v", err))
		return nil, "", fmt.Errorf("cannot fetch matching tuples: %v", err)
	}
	tuples := make([]TimestampedTuple, 0, len(resp.GetTuples()))
	for _, oTuple := range resp.GetTuples() {
		t, err := fromOpenFGATupleKey(*oTuple.Key)
		if err != nil {
			zapctx.Error(ctx, fmt.Sprintf("cannot parse tuple from Read response: %v", err))
			return nil, "", fmt.Errorf("cannot parse tuple %+v, %v", oTuple, err)
		}
		tuples = append(tuples, TimestampedTuple{
			Tuple:     t,
			Timestamp: *oTuple.Timestamp,
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

	er := openfga.NewExpandRequest(tuple.toOpenFGATuple())
	er.SetAuthorizationModelId(c.AuthModelId)
	resp, _, err := c.api.Expand(ctx).Body(*er).Execute()
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
		users, err := c.expand(ctx, maxDepth, *leaf.Users.Users...)
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
		if tupleToUserSet.HasComputed() {
			return c.expandComputed(ctx, maxDepth, leaf, tupleToUserSet.GetComputed()...)
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
		if computed.HasUserset() {
			userSet := computed.GetUserset()
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
			t := openfga.NewTupleKey()
			t.SetRelation(tokens[1])
			t.SetObject(tokens[0])
			tuple, err := fromOpenFGATupleKey(*t)
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
	lor.SetAuthorizationModelId(c.AuthModelId)
	lor.SetUser(tuple.Object.String())
	lor.SetRelation(tuple.Relation.String())
	lor.SetType(tuple.Target.Kind.String())

	if len(contextualTuples) > 0 {
		keys := make([]openfga.TupleKey, 0, len(contextualTuples))

		for _, ct := range contextualTuples {
			keys = append(keys, ct.toOpenFGATuple())
		}

		lor.SetContextualTuples(*openfga.NewContextualTupleKeys(keys))
	}

	resp, _, err := c.api.ListObjects(ctx).Body(*lor).Execute()
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
