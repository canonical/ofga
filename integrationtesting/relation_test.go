// Copyright 2025 Canonical Ltd.
// Licensed under the LGPL license, see LICENSE file for details.

package integration

import (
	"context"
	"testing"

	"github.com/canonical/ofga"
)

func TestIntegrationAddRelationIdempotent(t *testing.T) {
	// Setup OpenFGA client and store
	fgaClient, storeID, _ := setupTestClient(t)
	defer func() {
		// Cleanup: delete the test store
		_, _ = fgaClient.DeleteStore(t.Context()).Execute()
	}()

	// Create ofga client wrapper
	ofgaClient, err := ofga.NewClient(
		t.Context(),
		ofga.OpenFGAParams{
			Scheme:  "http",
			Host:    "localhost",
			Port:    "8080",
			StoreID: storeID,
		},
	)
	if err != nil {
		t.Fatalf("Failed to create OpenFGA client: %v", err)
	}
	// Test: Add relations multiple times
	addTuples := []ofga.Tuple{{
		Object:   &ofga.Entity{Kind: "user", ID: "456"},
		Relation: "editor",
		Target:   &ofga.Entity{Kind: "document", ID: "ABC"},
	}}
	// Add tuples idempotently.
	err = ofgaClient.AddRemoveRelationsIdempotent(t.Context(), addTuples, nil)
	if err != nil {
		t.Fatalf("Failed to add/remove relations idempotently: %v", err)
	}
	// Add tuple not idempotently should return an err.
	err = ofgaClient.AddRemoveRelations(t.Context(), addTuples, nil)
	if err == nil {
		t.Fatalf("Expected error when adding duplicate relations, but got none")
	}

	// Add tuples idempotently, shouldn't return an err even if they already exist.
	err = ofgaClient.AddRemoveRelationsIdempotent(t.Context(), addTuples, nil)
	if err != nil {
		t.Fatalf("Failed to add/remove relations idempotently: %v", err)
	}
}

func TestIntegrationAddRelationIdempotentSameRequest(t *testing.T) {
	// Setup OpenFGA client and store
	fgaClient, storeID, _ := setupTestClient(t)
	defer func() {
		// Cleanup: delete the test store
		_, _ = fgaClient.DeleteStore(t.Context()).Execute()
	}()

	// Create ofga client wrapper
	ofgaClient, err := ofga.NewClient(
		t.Context(),
		ofga.OpenFGAParams{
			Scheme:  "http",
			Host:    "localhost",
			Port:    "8080",
			StoreID: storeID,
		},
	)
	if err != nil {
		t.Fatalf("Failed to create OpenFGA client: %v", err)
	}
	// Test: Add relations multiple times
	addTuples := []ofga.Tuple{{
		Object:   &ofga.Entity{Kind: "user", ID: "456"},
		Relation: "editor",
		Target:   &ofga.Entity{Kind: "document", ID: "ABC"},
	}, {
		Object:   &ofga.Entity{Kind: "user", ID: "456"},
		Relation: "editor",
		Target:   &ofga.Entity{Kind: "document", ID: "ABC"},
	}}
	// Add tuples idempotently.
	err = ofgaClient.AddRemoveRelationsIdempotent(t.Context(), addTuples, nil)
	if err == nil {
		t.Fatalf("Expected error when adding duplicate relations in the same request, but got none")
	}
}

func TestIntegrationRemoveRelationIdempotent(t *testing.T) {
	// Setup OpenFGA client and store
	fgaClient, storeID, _ := setupTestClient(t)
	defer func() {
		// Cleanup: delete the test store
		ctx := context.Background()
		_, _ = fgaClient.DeleteStore(ctx).Execute()
	}()

	// Create ofga client wrapper
	ofgaClient, err := ofga.NewClient(
		t.Context(),
		ofga.OpenFGAParams{
			Scheme:  "http",
			Host:    "localhost",
			Port:    "8080",
			StoreID: storeID,
		},
	)
	if err != nil {
		t.Fatalf("Failed to create OpenFGA client: %v", err)
	}
	// Test: Remove relations not existing
	removeTuples := []ofga.Tuple{{
		Object:   &ofga.Entity{Kind: "user", ID: "456"},
		Relation: "editor",
		Target:   &ofga.Entity{Kind: "document", ID: "ABC"},
	}}
	// Remove tuples idempotently.
	err = ofgaClient.AddRemoveRelationsIdempotent(t.Context(), nil, removeTuples)
	if err != nil {
		t.Fatalf("Failed to add/remove relations idempotently: %v", err)
	}
	// Remove tuple not idempotently should return an err.
	err = ofgaClient.AddRemoveRelations(t.Context(), nil, removeTuples)
	if err == nil {
		t.Fatalf("Expected error when adding duplicate relations, but got none")
	}
}

func TestIntegrationCheckRelationWithConditionContext(t *testing.T) {
	// Setup OpenFGA client and store
	fgaClient, storeID, modelID := setupTestClient(t)
	defer func() {
		// Cleanup: delete the test store
		_, _ = fgaClient.DeleteStore(t.Context()).Execute()
	}()

	// Create ofga client wrapper
	ofgaClient, err := ofga.NewClient(
		t.Context(),
		ofga.OpenFGAParams{
			Scheme:      "http",
			Host:        "localhost",
			Port:        "8080",
			StoreID:     storeID,
			AuthModelID: modelID,
		},
	)
	if err != nil {
		t.Fatalf("Failed to create OpenFGA client: %v", err)
	}

	checkTuple := ofga.Tuple{
		Object:   &ofga.Entity{Kind: "user", ID: "alice"},
		Relation: "viewer",
		Target:   &ofga.Entity{Kind: "report", ID: "quarterly"},
	}
	contextualTuple := checkTuple
	contextualTuple.Condition = ofga.NewRelationshipCondition("can_grant")

	allowed, err := ofgaClient.CheckRelationWithContext(
		t.Context(),
		checkTuple,
		map[string]any{"grantable": true},
		contextualTuple,
	)
	if err != nil {
		t.Fatalf("Failed to check relation with condition context: %v", err)
	}
	if !allowed {
		t.Fatalf("Expected conditioned contextual tuple to allow access when grantable is true")
	}

	allowed, err = ofgaClient.CheckRelationWithContext(
		t.Context(),
		checkTuple,
		map[string]any{"grantable": false},
		contextualTuple,
	)
	if err != nil {
		t.Fatalf("Failed to check relation with condition context: %v", err)
	}
	if allowed {
		t.Fatalf("Expected conditioned contextual tuple to deny access when grantable is false")
	}
}
