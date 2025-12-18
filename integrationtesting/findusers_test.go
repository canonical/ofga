package integration

import (
	"sort"
	"testing"

	"github.com/canonical/ofga"
)

func TestFindUsersByRelationWithTupleToUserset(t *testing.T) {
	// Setup OpenFGA client and store
	fgaClient, storeID, _ := setupTestClient(t)
	defer func() {
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

	// Setup relationships with tuple-to-userset pattern:
	// 1. doc1 has parent folder1
	// 2. org1#member is editor of folder1
	// 3. alice is member of org1
	// 4. bob is member of org2
	// 5. org2#member is member of org1 (nested)
	// 6. eve is editor of doc1
	//
	// This creates: doc1 viewer -> editor (from parent folder1) -> org1#member -> org2#member -> bob
	// The tupleToUserset expansion of "editor from parent" should hit the bug fix

	tuples := []ofga.Tuple{
		// doc1 has parent folder1
		{
			Object:   &ofga.Entity{Kind: "folder", ID: "folder1"},
			Relation: "parent",
			Target:   &ofga.Entity{Kind: "document", ID: "doc1"},
		},
		// org1#member is editor of folder1
		{
			Object: &ofga.Entity{
				Kind:     "organization",
				ID:       "org1",
				Relation: "member",
			},
			Relation: "editor",
			Target:   &ofga.Entity{Kind: "folder", ID: "folder1"},
		},
		// alice is member of org1
		{
			Object:   &ofga.Entity{Kind: "user", ID: "alice"},
			Relation: "member",
			Target:   &ofga.Entity{Kind: "organization", ID: "org1"},
		},
		// bob is member of org2
		{
			Object:   &ofga.Entity{Kind: "user", ID: "bob"},
			Relation: "member",
			Target:   &ofga.Entity{Kind: "organization", ID: "org2"},
		},
		// org2#member is member of org1 (nested)
		{
			Object: &ofga.Entity{
				Kind:     "organization",
				ID:       "org2",
				Relation: "member",
			},
			Relation: "member",
			Target:   &ofga.Entity{Kind: "organization", ID: "org1"},
		},
		// eve is editor of doc1
		{
			Object:   &ofga.Entity{Kind: "user", ID: "eve"},
			Relation: "editor",
			Target:   &ofga.Entity{Kind: "document", ID: "doc1"},
		},
	}

	err = ofgaClient.AddRelation(t.Context(), tuples...)
	if err != nil {
		t.Fatalf("Failed to add relations: %v", err)
	}

	// Test: Find all users with viewer relation to doc1
	users, err := ofgaClient.FindUsersByRelation(t.Context(), ofga.Tuple{
		Relation: "viewer",
		Target:   &ofga.Entity{Kind: "document", ID: "doc1"},
	}, 10)

	if err != nil {
		t.Fatalf("FindUsersByRelation failed: %v", err)
	}

	// Extract user IDs and sort for comparison
	userIDs := make([]string, len(users))
	for i, user := range users {
		userIDs[i] = user.ID
	}
	sort.Strings(userIDs)

	expected := []string{"alice", "bob", "eve"}
	if len(userIDs) != len(expected) {
		t.Fatalf("Expected %d users, got %d: %v", len(expected), len(userIDs), userIDs)
	}
	for i, expectedID := range expected {
		if userIDs[i] != expectedID {
			t.Errorf("Expected user %s at position %d, got %s", expectedID, i, userIDs[i])
		}
	}
}
