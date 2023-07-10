package ofga_test

import (
	"context"
	"fmt"
	"os"

	"github.com/canonical/ofga"
)

func ExampleParseEntity() {
	entity, err := ofga.ParseEntity("organization:canonical")
	fmt.Printf("%+v %v", entity, err)
	// Output:
	// {Kind:organization ID:canonical Relation:} <nil>
}

func ExampleParseEntity_relation() {
	entity, err := ofga.ParseEntity("organization:canonical#member")
	fmt.Printf("%+v %v", entity, err)
	// Output:
	// {Kind:organization ID:canonical Relation:member} <nil>
}

func ExampleClient_AddRelation() {
	// Initialize the client
	ctx := context.Background()
	client, err := ofga.NewClient(ctx, ofga.OpenFGAParams{
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

	// Add a relationship tuple
	err = client.AddRelation(ctx, ofga.Tuple{
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
	// Initialize the client
	ctx := context.Background()
	client, err := ofga.NewClient(ctx, ofga.OpenFGAParams{
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

	// Add relationship tuples
	err = client.AddRelation(ctx,
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
	// Initialize the client
	ctx := context.Background()
	client, err := ofga.NewClient(ctx, ofga.OpenFGAParams{
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

	// Check if the relation exists
	allowed, err := client.CheckRelation(ctx, ofga.Tuple{
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
	// Initialize the client
	ctx := context.Background()
	client, err := ofga.NewClient(ctx, ofga.OpenFGAParams{
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

	contextualTuples := []ofga.Tuple{{
		Object:   &ofga.Entity{Kind: "user", ID: "123"},
		Relation: "editor",
		Target:   &ofga.Entity{Kind: "document", ID: "ABC"},
	}}

	// Check if the relation exists
	allowed, err := client.CheckRelation(ctx, ofga.Tuple{
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
