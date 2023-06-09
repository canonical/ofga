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
	"strings"

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
		zapctx.Error(ctx, fmt.Sprintf("cannot execute Write request %q", err))
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
		zapctx.Error(ctx, fmt.Sprintf("cannot execute Check request %q", err))
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
		zapctx.Error(ctx, fmt.Sprintf("cannot execute Write request %q", err))
		return err
	}
	return nil
}

// CreateStore creates a new store on the openFGA instance and returns its ID.
func (c *Client) CreateStore(ctx context.Context, name string) (string, error) {
	csr := openfga.NewCreateStoreRequest(name)
	resp, _, err := c.api.CreateStore(ctx).Body(*csr).Execute()
	if err != nil {
		zapctx.Error(ctx, fmt.Sprintf("cannot execute CreateStore request %q", err))
		return "", fmt.Errorf("cannot list stores %q", err)
	}
	return resp.GetId(), nil
}

// ListStores returns the list of stores present on the openFGA instance. If
// pageSize is set to 0, then the default pageSize is used. If this is the
// initial request, an empty string can be passed in as the paginationToken.
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
		zapctx.Error(ctx, fmt.Sprintf("cannot execute ListStores request %q", err))
		return nil, fmt.Errorf("cannot list stores %q", err)
	}
	return resp.GetStores(), nil
}

// ReadChanges returns a paginated list of tuple changes (additions and
// deletions) sorted by ascending time. The response will include a continuation
// token that is used to get the next set of changes. If there are no changes
// after the provided continuation token, the same token will be returned in
// order for it to be used when new changes are recorded. If no tuples have been
// added or removed, this token will be empty. The entityType parameter can be
// used to restrict the response to show only changes affecting a specific type.
func (c *Client) ReadChanges(ctx context.Context, entityType string, pageSize int32, paginationToken string) (openfga.ReadChangesResponse, error) {
	rcr := c.api.ReadChanges(ctx)
	rcr.Type_(entityType)
	if pageSize != 0 {
		rcr = rcr.PageSize(pageSize)
	}
	if paginationToken != "" {
		rcr = rcr.ContinuationToken(paginationToken)
	}

	resp, _, err := rcr.Execute()
	if err != nil {
		zapctx.Error(ctx, fmt.Sprintf("cannot execute ReadChanges request %q", err))
		return openfga.ReadChangesResponse{}, fmt.Errorf("cannot read changes %q", err)
	}
	return resp, nil
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
		zapctx.Error(ctx, fmt.Sprintf("cannot execute WriteAuthorizationModel request %q", err))
		return "", err
	}
	return resp.GetAuthorizationModelId(), nil
}

// ListAuthModels returns the list of authorization models present on the
// openFGA instance.
func (c *Client) ListAuthModels(ctx context.Context) ([]openfga.AuthorizationModel, error) {
	resp, _, err := c.api.ReadAuthorizationModels(ctx).Execute()
	if err != nil {
		zapctx.Error(ctx, fmt.Sprintf("cannot execute ReadAuthorizationModels request %q", err))
		return nil, fmt.Errorf("cannot list authorization models %q", err)
	}
	return resp.GetAuthorizationModels(), nil
}

// GetAuthModel fetches an authorization model by ID from the openFGA instance.
func (c *Client) GetAuthModel(ctx context.Context, ID string) (openfga.AuthorizationModel, error) {
	resp, _, err := c.api.ReadAuthorizationModel(ctx, ID).Execute()
	if err != nil {
		zapctx.Error(ctx, fmt.Sprintf("cannot execute ReadAuthorizationModel request %q", err))
		return openfga.AuthorizationModel{}, fmt.Errorf("cannot list authorization models %q", err)
	}
	return resp.GetAuthorizationModel(), nil
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
func (c *Client) FindMatchingTuples(ctx context.Context, tuple Tuple, pageSize int32, paginationToken string) ([]TimestampedTuple, error) {
	rr := openfga.NewReadRequest()
	if !tuple.isEmpty() {
		if tuple.Target.Kind == "" {
			return nil, errors.New("missing tuple.Target.Kind")
		}
		if tuple.Target.ID == "" && (tuple.Object.Kind == "" || tuple.Object.ID == "") {
			return nil, errors.New("either tuple.Target.ID or tuple.Object must be specified")
		}
		if tuple.Target.Relation != "" {
			return nil, errors.New("invalid tuple.Target, tuple.Target.Relation must not be set")
		}
		rr.SetTupleKey(tuple.toOpenFGATuple())
	}
	if pageSize != 0 {
		rr.SetPageSize(pageSize)
	}
	if paginationToken != "" {
		rr.SetContinuationToken(paginationToken)
	}
	resp, _, err := c.api.Read(ctx).Body(*rr).Execute()
	if err != nil {
		zapctx.Error(ctx, fmt.Sprintf("cannot execute Read request %q", err))
		return nil, err
	}
	tuples := make([]TimestampedTuple, len(resp.GetTuples()))
	for _, oTuple := range resp.GetTuples() {
		t, err := fromOpenFGATupleKey(*oTuple.Key)
		if err != nil {
			zapctx.Error(ctx, fmt.Sprintf("cannot parse tuple from Read response %q", err))
			return nil, fmt.Errorf("cannot parse tuple %+v. %w", oTuple, err)
		}
		tuples = append(tuples, TimestampedTuple{
			tuple:     t,
			timestamp: *oTuple.Timestamp,
		})
	}
	return tuples, nil
}

// FindUsersByRelation fetches the list of users that have a specific
// relation with a specific target object. This method not only searches
// through the relationship tuples present in the system, but also takes into
// consideration the authorization model and the relationship tuples implied
// by the model (for instance, a writer of a document is also a viewer of
// the document), and recursively expands these relationships to obtain the
// final list of users.
//
// This method requires that Tuple.Target and Tuple.Relation be specified.
//
// Note that this method call is expensive, and should be used with caution.
func (c *Client) FindUsersByRelation(ctx context.Context, tuple Tuple) ([]Entity, error) {
	userStrings, err := c.findUsersWithRelation(ctx, tuple)
	if err != nil {
		return nil, err
	}
	var users []Entity
	for u := range userStrings {
		user, err := ParseEntity(u)
		if err != nil {
			return nil, fmt.Errorf("cannot parse entity %s from Expand response %q", u, err)
		}
		users = append(users, user)
	}
	return users, nil
}

// findUsersWithRelation is the internal implementation for
// FindUsersByRelation. It returns a set of userStrings representing the
// list of users that have access to the specified object via the specified
// relation.
func (c *Client) findUsersWithRelation(ctx context.Context, tuple Tuple) (map[string]bool, error) {
	if tuple.Target.Kind == "" || tuple.Target.ID == "" {
		return nil, errors.New("missing tuple.Target")
	}
	if tuple.Target.Relation != "" {
		return nil, errors.New("invalid tuple.Target, tuple.Target.Relation must not be set")
	}
	if tuple.Relation == "" {
		return nil, errors.New("missing tuple.Relation")
	}

	er := openfga.NewExpandRequest(tuple.toOpenFGATuple())
	er.SetAuthorizationModelId(c.AuthModelId)
	res, _, err := c.api.Expand(ctx).Body(*er).Execute()
	if err != nil {
		zapctx.Error(ctx, fmt.Sprintf("cannot execute Expand request %q", err))
		return nil, err
	}

	tree := res.GetTree()
	if !tree.HasRoot() {
		return nil, errors.New("unexpected tree structure from Expand response")
	}
	root := tree.GetRoot()
	leaves, err := c.traverseTree(ctx, &root)
	if err != nil {
		return nil, err
	}
	return leaves, nil
}

// traverseTree will recursively expand the tree returned by an openfga Expand
// call to find all users that have the specified relation to the specified
// target entity.
func (c *Client) traverseTree(ctx context.Context, node *openfga.Node) (map[string]bool, error) {
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
			childNodeUsers, err := c.traverseTree(ctx, &childNode)
			if err != nil {
				return nil, err
			}
			for userString := range childNodeUsers {
				users[userString] = true
			}
		}
		return users, nil
	}
	if node.HasLeaf() {
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
		// If the leaf node contains ComputedSets or TupleToEntitySets, we need
		// to expand them further to obtain individual users.
		if leaf.HasUsers() {
			users, err := c.expand(ctx, *leaf.Users.Users...)
			if err != nil {
				return nil, err
			}
			return users, nil
		} else if leaf.HasComputed() {
			computed := leaf.GetComputed()
			if computed.HasUserset() {
				userSet := computed.GetUserset()
				users, err := c.expand(ctx, userSet)
				if err != nil {
					return nil, err
				}
				return users, nil
			} else {
				logError("missing userSet", "leaf", leaf)
				return nil, errors.New("missing userSet")
			}
		} else if leaf.HasTupleToUserset() {
			tupleToUserSet := leaf.GetTupleToUserset()
			if tupleToUserSet.HasComputed() {
				computedList := tupleToUserSet.GetComputed()
				users := make(map[string]bool)
				// We're interested in the list of computed nodes that
				// this TupleToUserSet contains. We need to expand each of these
				// to get the leaf users.
				for _, computed := range computedList {
					if computed.HasUserset() {
						userSet := computed.GetUserset()
						found, err := c.expand(ctx, userSet)
						if err != nil {
							return nil, err
						}
						for userString := range found {
							users[userString] = true
						}
					} else {
						logError("tupleToUserSet: missing userSet", "leaf", leaf)
						return nil, errors.New("missing userSet")
					}
				}
				return users, nil
			}
		} else {
			logError("unknown leaf type", "leaf", leaf)
			return nil, errors.New("unknown leaf type")
		}
	}
	logError("unknown node type", "node", node)
	return nil, errors.New("unknown node type")
}

// expand checks all userStrings in the input list and expands any userSets
// that are present into the constituent individual users. Example:
// "team:planning#members" would be expanded into "user:abc", "user:xyz", etc.
func (c *Client) expand(ctx context.Context, userStrings ...string) (map[string]bool, error) {
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
				return nil, errors.New("failed to parse tuple")
			}
			found, err := c.findUsersWithRelation(ctx, tuple)
			if err != nil {
				return nil, err
			}
			for userString := range found {
				users[userString] = true
			}
		default:
			zapctx.Error(ctx, fmt.Sprintf("unknown user representation %s", u))
			return nil, errors.New("unknown user representation")
		}
	}
	return users, nil
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
	if tuple.Object.Kind == "" || tuple.Object.ID == "" {
		return nil, errors.New("missing tuple.Object")
	}
	if tuple.Relation == "" {
		return nil, errors.New("missing tuple.Relation")
	}
	if tuple.Target.Kind == "" || tuple.Target.Relation != "" || tuple.Target.ID != "" {
		return nil, errors.New("invalid tuple.Target, only tuple.Target.Kind must be set")
	}

	lor := openfga.NewListObjectsRequestWithDefaults()
	lor.SetAuthorizationModelId(c.AuthModelId)
	lor.SetUser(tuple.Object.String())
	lor.SetRelation(tuple.Relation.String())
	lor.SetType(tuple.Target.Kind.String())

	if len(contextualTuples) > 0 {
		keys := make([]openfga.TupleKey, len(contextualTuples))

		for i, ct := range contextualTuples {
			keys[i] = ct.toOpenFGATuple()
		}

		lor.SetContextualTuples(*openfga.NewContextualTupleKeys(keys))
	}

	resp, _, err := c.api.ListObjects(ctx).Body(*lor).Execute()
	if err != nil {
		return nil, err
	}

	objects := make([]Entity, len(resp.GetObjects()))
	for _, o := range resp.GetObjects() {
		e, err := ParseEntity(o)
		if err != nil {
			return nil, fmt.Errorf("cannot parse entity %s from ListObjects response %q", o, err)
		}
		objects = append(objects, e)
	}

	return objects, nil
}
