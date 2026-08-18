// Copyright 2025 Canonical Ltd.
// Licensed under the LGPL license, see LICENSE file for details.

package integration

import (
	"strconv"
	"testing"

	"github.com/canonical/ofga"
)

func TestIntegrationBatchCheck(t *testing.T) {
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
	addTuples := []ofga.Tuple{}
	// Test: Add relations multiple times
	for i := range 50 {
		addTuples = append(addTuples, ofga.Tuple{
			Object:   &ofga.Entity{Kind: "user", ID: strconv.Itoa(i)},
			Relation: "editor",
			Target:   &ofga.Entity{Kind: "document", ID: "ABC"},
		})
	}
	// Add tuples idempotently.
	err = ofgaClient.AddRemoveRelationsIdempotent(t.Context(), addTuples, nil)
	if err != nil {
		t.Fatalf("Failed to add/remove relations idempotently: %v", err)
	}
	batchTuples := make([]ofga.TupleWithCorrelationId, 0, len(addTuples))
	for _, tuple := range addTuples {
		batchTuples = append(batchTuples, ofga.TupleWithCorrelationId{
			Tuple:         &tuple,
			CorrelationId: "correlation_" + tuple.Object.ID,
		})
	}
	results, err := ofgaClient.BatchCheckRelations(t.Context(), batchTuples)
	if err != nil {
		t.Fatalf("Failed to batch check relations: %v", err)
	}
	// Verify the results
	for _, tuple := range batchTuples {
		result, exists := results[tuple.CorrelationId]
		if !exists {
			t.Errorf("No result found for correlation ID: %s", tuple.CorrelationId)
		} else if !result {
			t.Errorf("Expected relation to exist for correlation ID: %s, but it does not", tuple.CorrelationId)
		}
	}
}

func TestIntegrationBatchCheckWithContextualTuples(t *testing.T) {
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

	// The relation only exists as a contextual (non-persistent) tuple.
	relation := ofga.Tuple{
		Object:   &ofga.Entity{Kind: "user", ID: "alice"},
		Relation: "editor",
		Target:   &ofga.Entity{Kind: "document", ID: "readme"},
	}

	// Without the contextual tuple, the relation should be denied.
	checkWithoutContext := ofga.TupleWithCorrelationId{
		Tuple:         &relation,
		CorrelationId: "without-context",
	}
	// With the contextual tuple attached to this item, it should be allowed.
	checkWithContext := ofga.TupleWithCorrelationId{
		Tuple:            &relation,
		CorrelationId:    "with-context",
		ContextualTuples: []ofga.Tuple{relation},
	}

	results, err := ofgaClient.BatchCheckRelations(t.Context(), []ofga.TupleWithCorrelationId{
		checkWithoutContext,
		checkWithContext,
	})
	if err != nil {
		t.Fatalf("Failed to batch check relations: %v", err)
	}

	if results[checkWithoutContext.CorrelationId] {
		t.Errorf("Expected relation to be denied without its contextual tuple")
	}
	if !results[checkWithContext.CorrelationId] {
		t.Errorf("Expected relation to be allowed with its contextual tuple")
	}
}

func TestIntegrationCheckMultipleRelations(t *testing.T) {
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
	addTuples := []ofga.Tuple{}
	// Test: Add relations multiple times
	for i := range 100 {
		addTuples = append(addTuples, ofga.Tuple{
			Object:   &ofga.Entity{Kind: "user", ID: strconv.Itoa(i)},
			Relation: "editor",
			Target:   &ofga.Entity{Kind: "document", ID: "ABC"},
		})
	}
	// Add tuples idempotently.
	err = ofgaClient.AddRemoveRelationsIdempotent(t.Context(), addTuples, nil)
	if err != nil {
		t.Fatalf("Failed to add/remove relations idempotently: %v", err)
	}
	for _, tuple := range addTuples {
		checked, err := ofgaClient.CheckRelation(t.Context(), tuple)
		if err != nil {
			t.Fatalf("Failed to check relation for tuple %+v: %v", tuple, err)
		}
		if !checked {
			t.Errorf("Expected relation to exist for tuple %+v, but it does not", tuple)
		}
	}
}
