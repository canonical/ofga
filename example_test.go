// Copyright 2023 Canonical Ltd.
// Licensed under the LGPL license, see LICENSE file for details.

package ofga_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/canonical/ofga"
)

var client *ofga.Client

func TestMain(m *testing.M) {
	client, _ = ofga.NewClient(context.Background(), ofga.OpenFGAParams{
		Scheme:      os.Getenv("OPENFGA_API_SCHEME"), // defaults to `https` if not specified.
		Host:        os.Getenv("OPENFGA_API_HOST"),
		Port:        os.Getenv("OPENFGA_API_PORT"),
		Token:       os.Getenv("SECRET_TOKEN"),          // Optional, based on the OpenFGA instance configuration.
		StoreID:     os.Getenv("OPENFGA_STORE_ID"),      // Required only when connecting to a pre-existing store.
		AuthModelID: os.Getenv("OPENFGA_AUTH_MODEL_ID"), // Required only when connecting to a pre-existing auth model.
	})
	os.Exit(m.Run())
}

func ExampleParseEntity() {
	entity, err := ofga.ParseEntity("organization:canonical")
	fmt.Printf("%+v %v", entity, err)
	// Output:
	// {Kind:organization ID:canonical Relation:} <nil>
}

func ExampleParseEntity_entitySet() {
	entity, err := ofga.ParseEntity("organization:canonical#member")
	fmt.Printf("%+v %v", entity, err)
	// Output:
	// {Kind:organization ID:canonical Relation:member} <nil>
}

func ExampleNewClient() {
	client, err := ofga.NewClient(context.Background(), ofga.OpenFGAParams{
		Scheme:      os.Getenv("OPENFGA_API_SCHEME"), // defaults to `https` if not specified.
		Host:        os.Getenv("OPENFGA_API_HOST"),
		Port:        os.Getenv("OPENFGA_API_PORT"),
		Token:       os.Getenv("SECRET_TOKEN"),          // Optional, based on the OpenFGA instance configuration.
		StoreID:     os.Getenv("OPENFGA_STORE_ID"),      // Required only when connecting to a pre-existing store.
		AuthModelID: os.Getenv("OPENFGA_AUTH_MODEL_ID"), // Required only when connecting to a pre-existing auth model.
	})
	if err != nil {
		// Handle err
		return
	}
	fmt.Print(client.GetAuthModelID())
}

func ExampleClient_AddRelation() {
	// Add a relationship tuple
	err := client.AddRelation(context.Background(), ofga.Tuple{
		Object:   &ofga.Entity{Kind: "user", ID: "123"},
		Relation: "editor",
		Target:   &ofga.Entity{Kind: "document", ID: "ABC"},
	})
	if err != nil {
		// Handle err
		return
	}
}

func ExampleClient_AddRelation_multiple() {
	// Add relationship tuples
	err := client.AddRelation(context.Background(),
		ofga.Tuple{
			Object:   &ofga.Entity{Kind: "user", ID: "123"},
			Relation: "editor",
			Target:   &ofga.Entity{Kind: "document", ID: "ABC"},
		},
		ofga.Tuple{
			Object:   &ofga.Entity{Kind: "user", ID: "456"},
			Relation: "editor",
			Target:   &ofga.Entity{Kind: "document", ID: "ABC"},
		},
		ofga.Tuple{
			Object:   &ofga.Entity{Kind: "user", ID: "789"},
			Relation: "editor",
			Target:   &ofga.Entity{Kind: "document", ID: "ABC"},
		},
	)
	if err != nil {
		// Handle err
		return
	}
}

func ExampleClient_CheckRelation() {
	// Check if the relation exists
	allowed, err := client.CheckRelation(context.Background(), ofga.Tuple{
		Object:   &ofga.Entity{Kind: "user", ID: "123"},
		Relation: "editor",
		Target:   &ofga.Entity{Kind: "document", ID: "ABC"},
	})
	if err != nil {
		// Handle err
		return
	}
	fmt.Printf("allowed: %v", allowed)
}

func ExampleClient_CheckRelation_contextualTuples() {
	contextualTuples := []ofga.Tuple{{
		Object:   &ofga.Entity{Kind: "user", ID: "123"},
		Relation: "editor",
		Target:   &ofga.Entity{Kind: "document", ID: "ABC"},
	}}

	// Check if the relation exists
	allowed, err := client.CheckRelation(context.Background(), ofga.Tuple{
		Object:   &ofga.Entity{Kind: "user", ID: "123"},
		Relation: "editor",
		Target:   &ofga.Entity{Kind: "document", ID: "ABC"},
	},
		contextualTuples...,
	)
	if err != nil {
		// Handle err
		return
	}
	fmt.Printf("allowed: %v", allowed)
}

func ExampleClient_CheckRelationWithTracing() {
	// Check if the relation exists
	allowed, err := client.CheckRelationWithTracing(context.Background(), ofga.Tuple{
		Object:   &ofga.Entity{Kind: "user", ID: "123"},
		Relation: "editor",
		Target:   &ofga.Entity{Kind: "document", ID: "ABC"},
	})
	if err != nil {
		// Handle err
		return
	}
	fmt.Printf("allowed: %v", allowed)
}

func ExampleClient_CheckRelationWithTracing_contextualTuples() {
	contextualTuples := []ofga.Tuple{{
		Object:   &ofga.Entity{Kind: "user", ID: "123"},
		Relation: "editor",
		Target:   &ofga.Entity{Kind: "document", ID: "ABC"},
	}}

	// Check if the relation exists
	allowed, err := client.CheckRelationWithTracing(context.Background(), ofga.Tuple{
		Object:   &ofga.Entity{Kind: "user", ID: "123"},
		Relation: "editor",
		Target:   &ofga.Entity{Kind: "document", ID: "ABC"},
	},
		contextualTuples...,
	)
	if err != nil {
		// Handle err
		return
	}
	fmt.Printf("allowed: %v", allowed)
}

func ExampleClient_RemoveRelation() {
	// Remove a relationship tuple
	err := client.RemoveRelation(context.Background(), ofga.Tuple{
		Object:   &ofga.Entity{Kind: "user", ID: "123"},
		Relation: "editor",
		Target:   &ofga.Entity{Kind: "document", ID: "ABC"},
	})
	if err != nil {
		// Handle err
		return
	}
}

func ExampleClient_RemoveRelation_multiple() {
	// Remove relationship tuples
	err := client.RemoveRelation(context.Background(),
		ofga.Tuple{
			Object:   &ofga.Entity{Kind: "user", ID: "123"},
			Relation: "editor",
			Target:   &ofga.Entity{Kind: "document", ID: "ABC"},
		},
		ofga.Tuple{
			Object:   &ofga.Entity{Kind: "user", ID: "456"},
			Relation: "editor",
			Target:   &ofga.Entity{Kind: "document", ID: "ABC"},
		},
		ofga.Tuple{
			Object:   &ofga.Entity{Kind: "user", ID: "789"},
			Relation: "editor",
			Target:   &ofga.Entity{Kind: "document", ID: "ABC"},
		},
	)
	if err != nil {
		// Handle err
		return
	}
}

func ExampleClient_AddRemoveRelations() {
	// Add and remove tuples to modify a user's relation with a document
	// from viewer to editor.
	addTuples := []ofga.Tuple{{
		Object:   &ofga.Entity{Kind: "user", ID: "456"},
		Relation: "editor",
		Target:   &ofga.Entity{Kind: "document", ID: "ABC"},
	}}
	removeTuples := []ofga.Tuple{{
		Object:   &ofga.Entity{Kind: "user", ID: "456"},
		Relation: "viewer",
		Target:   &ofga.Entity{Kind: "document", ID: "ABC"},
	}}
	// Add and remove tuples atomically.
	err := client.AddRemoveRelations(context.Background(), addTuples, removeTuples)
	if err != nil {
		// Handle err
		return
	}
}

func ExampleClient_CreateStore() {
	// Create a store named "Alpha"
	storeID, err := client.CreateStore(context.Background(), "Alpha")
	if err != nil {
		// Handle err
		return
	}
	fmt.Println(storeID)
}

func ExampleClient_ListStores() {
	// Fetch a list of stores using the default page size
	resp, err := client.ListStores(context.Background(), 0, "")
	if err != nil {
		// Handle err
		return
	}
	for _, store := range resp.GetStores() {
		// Processing
		fmt.Println(store)
	}

	// If it exists, fetch the next page of stores
	if resp.HasContinuationToken() {
		resp, err = client.ListStores(context.Background(), 0, resp.GetContinuationToken())
		if err != nil {
			// Handle err
			return
		}
		for _, store := range resp.GetStores() {
			// Processing
			fmt.Println(store)
		}
	}
}

func ExampleClient_ReadChanges() {
	// Fetch all tuple changes since the start. Use the default page_size.
	resp, err := client.ReadChanges(context.Background(), "", 0, "")
	if err != nil {
		// Handle err
		return
	}
	for _, change := range resp.GetChanges() {
		// Processing
		fmt.Println(change)
	}
}

func ExampleClient_ReadChanges_forSpecificEntityType() {
	// Fetch all tuple changes affecting organizations since the start.
	// Use the default page_size.
	resp, err := client.ReadChanges(context.Background(), "organization", 0, "")
	if err != nil {
		// Handle err
		return
	}
	for _, change := range resp.GetChanges() {
		// Processing
		fmt.Println(change)
	}
}

func ExampleAuthModelFromJSON() {
	// Assume we have the following auth model
	json := []byte(`{
	  "type_definitions": [
		{
		  "type": "user",
		  "relations": {}
		}
	  ],
	  "schema_version": "1.1"
	}`)

	// Convert json into internal representation
	model, err := ofga.AuthModelFromJSON(json)
	if err != nil {
		// Handle err
	}
	// Use the model
	fmt.Println(model)
}

func ExampleClient_CreateAuthModel() {
	// Assume we have the following json auth model
	json := []byte(`{
	  "type_definitions": [
		{
		  "type": "user",
		  "relations": {}
		}
	  ],
	  "schema_version": "1.1"
	}`)

	// Convert json into internal representation
	model, err := ofga.AuthModelFromJSON(json)
	if err != nil {
		// Handle err
	}

	// Create an auth model in OpenFGA using the internal representation
	authModelID, err := client.CreateAuthModel(context.Background(), model)
	if err != nil {
		// Handle error
	}
	fmt.Println(authModelID)
}

func ExampleClient_ListAuthModels() {
	// Fetch a list of auth models using the default page size
	resp, err := client.ListAuthModels(context.Background(), 0, "")
	if err != nil {
		// Handle err
		return
	}
	for _, model := range resp.GetAuthorizationModels() {
		// Processing
		fmt.Println(model.GetId())
	}

	// If it exists, fetch the next page of auth models
	if resp.HasContinuationToken() {
		resp, err = client.ListAuthModels(context.Background(), 0, resp.GetContinuationToken())
		if err != nil {
			// Handle err
			return
		}
		for _, model := range resp.GetAuthorizationModels() {
			// Processing
			fmt.Println(model.GetId())
		}
	}
}

func ExampleClient_GetAuthModel() {
	// fetch an auth model by ID
	model, err := client.GetAuthModel(context.Background(), "ABC1234")
	if err != nil {
		// Use the model
		fmt.Println(model)
	}
}

func ExampleClient_FindMatchingTuples() {
	// Find all tuples where bob is a writer of a document
	searchTuple := ofga.Tuple{
		Object:   &ofga.Entity{Kind: "user", ID: "bob"},
		Relation: "writer",
		Target:   &ofga.Entity{Kind: "document"},
	}

	tuples, continuationToken, err := client.FindMatchingTuples(context.Background(), searchTuple, 0, "")
	if err != nil {
		// Handle error
	}

	for _, tuple := range tuples {
		// Process the matching tuples
		fmt.Println(tuple)
	}

	// If required, fetch the next tuples using the continuation token
	if continuationToken != "" {
		tuples, continuationToken, err = client.FindMatchingTuples(context.Background(), searchTuple, 0, continuationToken)
		if err != nil {
			// Handle error
		}

		for _, tuple := range tuples {
			// Process the matching tuples
			fmt.Println(tuple)
		}
	}
}

func ExampleClient_FindUsersByRelation() {
	// Find all users that have the viewer relation with document ABC, expanding
	// matching user sets upto two levels deep only.
	searchTuple := ofga.Tuple{
		Relation: "viewer",
		Target:   &ofga.Entity{Kind: "document", ID: "ABC"},
	}
	users, err := client.FindUsersByRelation(context.Background(), searchTuple, 2)
	if err != nil {
		// Handle error
	}

	for _, user := range users {
		// Process the matching users
		fmt.Println(user)
	}
}

func ExampleClient_FindAccessibleObjectsByRelation() {
	// Find all documents that the user bob can view by virtue of direct or
	// implied relationships.
	searchTuple := ofga.Tuple{
		Object:   &ofga.Entity{Kind: "user", ID: "bob"},
		Relation: "viewer",
		Target:   &ofga.Entity{Kind: "document"},
	}
	docs, err := client.FindAccessibleObjectsByRelation(context.Background(), searchTuple)
	if err != nil {
		// Handle error
	}

	for _, doc := range docs {
		// Process the matching documents
		fmt.Println(doc)
	}
}
