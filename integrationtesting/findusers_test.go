package integration

import (
	"sort"
	"testing"

	"github.com/canonical/ofga"
)

func TestFindUsersByRelation(t *testing.T) {
	tests := []struct {
		name          string
		tuples        []ofga.Tuple
		query         ofga.Tuple
		maxDepth      int
		expectedUsers []ofga.Entity
		expectError   bool
		description   string
	}{
		{
			name:        "direct_user_relation",
			description: "Find users with direct editor relation to a document",
			tuples: []ofga.Tuple{
				{
					Object:   &ofga.Entity{Kind: "user", ID: "alice"},
					Relation: "editor",
					Target:   &ofga.Entity{Kind: "document", ID: "doc1"},
				},
				{
					Object:   &ofga.Entity{Kind: "user", ID: "bob"},
					Relation: "editor",
					Target:   &ofga.Entity{Kind: "document", ID: "doc1"},
				},
			},
			query: ofga.Tuple{
				Relation: "editor",
				Target:   &ofga.Entity{Kind: "document", ID: "doc1"},
			},
			maxDepth: 10,
			expectedUsers: []ofga.Entity{
				{Kind: "user", ID: "alice"},
				{Kind: "user", ID: "bob"},
			},
			expectError: false,
		},
		{
			name:        "tuple_to_userset_with_parent",
			description: "Find users through tuple-to-userset pattern with parent folder inheritance",
			tuples: []ofga.Tuple{
				// doc1 has parent folder1
				{
					Object:   &ofga.Entity{Kind: "folder", ID: "folder1"},
					Relation: "parent",
					Target:   &ofga.Entity{Kind: "document", ID: "doc1"},
				},
				// alice is editor of folder1
				{
					Object:   &ofga.Entity{Kind: "user", ID: "alice"},
					Relation: "editor",
					Target:   &ofga.Entity{Kind: "folder", ID: "folder1"},
				},
				// bob is viewer of doc1 directly
				{
					Object:   &ofga.Entity{Kind: "user", ID: "bob"},
					Relation: "viewer",
					Target:   &ofga.Entity{Kind: "document", ID: "doc1"},
				},
			},
			query: ofga.Tuple{
				Relation: "viewer",
				Target:   &ofga.Entity{Kind: "document", ID: "doc1"},
			},
			maxDepth: 10,
			expectedUsers: []ofga.Entity{
				{Kind: "user", ID: "alice"},
				{Kind: "user", ID: "bob"},
			},
			expectError: false,
		},
		{
			name:        "nested_organization_members",
			description: "Find users through nested organization memberships",
			tuples: []ofga.Tuple{
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
			},
			query: ofga.Tuple{
				Relation: "member",
				Target:   &ofga.Entity{Kind: "organization", ID: "org1"},
			},
			maxDepth: 10,
			expectedUsers: []ofga.Entity{
				{Kind: "user", ID: "alice"},
				{Kind: "user", ID: "bob"},
			},
			expectError: false,
		},
		{
			name:        "complex_nested_with_tuple_to_userset",
			description: "Complex scenario: nested organizations with tuple-to-userset through parent folder",
			tuples: []ofga.Tuple{
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
				// eve is editor of doc1 directly
				{
					Object:   &ofga.Entity{Kind: "user", ID: "eve"},
					Relation: "editor",
					Target:   &ofga.Entity{Kind: "document", ID: "doc1"},
				},
			},
			query: ofga.Tuple{
				Relation: "viewer",
				Target:   &ofga.Entity{Kind: "document", ID: "doc1"},
			},
			maxDepth: 10,
			expectedUsers: []ofga.Entity{
				{Kind: "user", ID: "alice"},
				{Kind: "user", ID: "bob"},
				{Kind: "user", ID: "eve"},
			},
			expectError: false,
		},
		{
			name:        "no_users_found",
			description: "Query for a relation with no users having access",
			tuples: []ofga.Tuple{
				{
					Object:   &ofga.Entity{Kind: "user", ID: "alice"},
					Relation: "viewer",
					Target:   &ofga.Entity{Kind: "document", ID: "doc1"},
				},
			},
			query: ofga.Tuple{
				Relation: "editor",
				Target:   &ofga.Entity{Kind: "document", ID: "doc1"},
			},
			maxDepth:      10,
			expectedUsers: []ofga.Entity{},
			expectError:   false,
		},
		{
			name:        "multiple_paths_same_user",
			description: "User has access through multiple paths (should appear only once)",
			tuples: []ofga.Tuple{
				// alice is editor directly
				{
					Object:   &ofga.Entity{Kind: "user", ID: "alice"},
					Relation: "editor",
					Target:   &ofga.Entity{Kind: "document", ID: "doc1"},
				},
				// alice is also viewer directly
				{
					Object:   &ofga.Entity{Kind: "user", ID: "alice"},
					Relation: "viewer",
					Target:   &ofga.Entity{Kind: "document", ID: "doc1"},
				},
			},
			query: ofga.Tuple{
				Relation: "viewer",
				Target:   &ofga.Entity{Kind: "document", ID: "doc1"},
			},
			maxDepth: 10,
			expectedUsers: []ofga.Entity{
				{Kind: "user", ID: "alice"},
			},
			expectError: false,
		},
		{
			name:        "deep_nested_organizations",
			description: "Multiple levels of organization nesting",
			tuples: []ofga.Tuple{
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
				// charlie is member of org3
				{
					Object:   &ofga.Entity{Kind: "user", ID: "charlie"},
					Relation: "member",
					Target:   &ofga.Entity{Kind: "organization", ID: "org3"},
				},
				// org2#member is member of org1
				{
					Object: &ofga.Entity{
						Kind:     "organization",
						ID:       "org2",
						Relation: "member",
					},
					Relation: "member",
					Target:   &ofga.Entity{Kind: "organization", ID: "org1"},
				},
				// org3#member is member of org2
				{
					Object: &ofga.Entity{
						Kind:     "organization",
						ID:       "org3",
						Relation: "member",
					},
					Relation: "member",
					Target:   &ofga.Entity{Kind: "organization", ID: "org2"},
				},
			},
			query: ofga.Tuple{
				Relation: "member",
				Target:   &ofga.Entity{Kind: "organization", ID: "org1"},
			},
			maxDepth: 10,
			expectedUsers: []ofga.Entity{
				{Kind: "user", ID: "alice"},
				{Kind: "user", ID: "bob"},
				{Kind: "user", ID: "charlie"},
			},
			expectError: false,
		},
		{
			name:        "multiple_documents_different_access",
			description: "Users have different access to different documents - verify isolation",
			tuples: []ofga.Tuple{
				// alice has editor access to doc1
				{
					Object:   &ofga.Entity{Kind: "user", ID: "alice"},
					Relation: "editor",
					Target:   &ofga.Entity{Kind: "document", ID: "doc1"},
				},
				// bob has editor access to doc2
				{
					Object:   &ofga.Entity{Kind: "user", ID: "bob"},
					Relation: "editor",
					Target:   &ofga.Entity{Kind: "document", ID: "doc2"},
				},
				// charlie has editor access to both doc1 and doc2
				{
					Object:   &ofga.Entity{Kind: "user", ID: "charlie"},
					Relation: "editor",
					Target:   &ofga.Entity{Kind: "document", ID: "doc1"},
				},
				{
					Object:   &ofga.Entity{Kind: "user", ID: "charlie"},
					Relation: "editor",
					Target:   &ofga.Entity{Kind: "document", ID: "doc2"},
				},
			},
			query: ofga.Tuple{
				Relation: "viewer",
				Target:   &ofga.Entity{Kind: "document", ID: "doc1"},
			},
			maxDepth: 10,
			expectedUsers: []ofga.Entity{
				{Kind: "user", ID: "alice"},
				{Kind: "user", ID: "charlie"},
			},
			expectError: false,
		},
		{
			name:        "multiple_documents_with_folders",
			description: "Multiple documents with parent folders - verify folder inheritance works per document",
			tuples: []ofga.Tuple{
				// doc1 has parent folder1
				{
					Object:   &ofga.Entity{Kind: "folder", ID: "folder1"},
					Relation: "parent",
					Target:   &ofga.Entity{Kind: "document", ID: "doc1"},
				},
				// doc2 has parent folder2
				{
					Object:   &ofga.Entity{Kind: "folder", ID: "folder2"},
					Relation: "parent",
					Target:   &ofga.Entity{Kind: "document", ID: "doc2"},
				},
				// doc3 has no parent
				// alice is editor of folder1
				{
					Object:   &ofga.Entity{Kind: "user", ID: "alice"},
					Relation: "editor",
					Target:   &ofga.Entity{Kind: "folder", ID: "folder1"},
				},
				// bob is editor of folder2
				{
					Object:   &ofga.Entity{Kind: "user", ID: "bob"},
					Relation: "editor",
					Target:   &ofga.Entity{Kind: "folder", ID: "folder2"},
				},
				// charlie is directly editor of doc3
				{
					Object:   &ofga.Entity{Kind: "user", ID: "charlie"},
					Relation: "editor",
					Target:   &ofga.Entity{Kind: "document", ID: "doc3"},
				},
			},
			query: ofga.Tuple{
				Relation: "viewer",
				Target:   &ofga.Entity{Kind: "document", ID: "doc1"},
			},
			maxDepth: 10,
			expectedUsers: []ofga.Entity{
				{Kind: "user", ID: "alice"},
			},
			expectError: false,
		},
		{
			name:        "multiple_documents_no_access_to_queried_doc",
			description: "Users have access to other documents but not the queried one",
			tuples: []ofga.Tuple{
				// alice has access to doc2 and doc3
				{
					Object:   &ofga.Entity{Kind: "user", ID: "alice"},
					Relation: "viewer",
					Target:   &ofga.Entity{Kind: "document", ID: "doc2"},
				},
				{
					Object:   &ofga.Entity{Kind: "user", ID: "alice"},
					Relation: "viewer",
					Target:   &ofga.Entity{Kind: "document", ID: "doc3"},
				},
				// bob has access to doc3
				{
					Object:   &ofga.Entity{Kind: "user", ID: "bob"},
					Relation: "viewer",
					Target:   &ofga.Entity{Kind: "document", ID: "doc3"},
				},
			},
			query: ofga.Tuple{
				Relation: "viewer",
				Target:   &ofga.Entity{Kind: "document", ID: "doc1"},
			},
			maxDepth:      10,
			expectedUsers: []ofga.Entity{},
			expectError:   false,
		},
		{
			name:        "multiple_documents_with_nested_orgs",
			description: "Complex scenario with multiple documents and nested organizations",
			tuples: []ofga.Tuple{
				// doc1 has parent folder1
				{
					Object:   &ofga.Entity{Kind: "folder", ID: "folder1"},
					Relation: "parent",
					Target:   &ofga.Entity{Kind: "document", ID: "doc1"},
				},
				// doc2 has parent folder2
				{
					Object:   &ofga.Entity{Kind: "folder", ID: "folder2"},
					Relation: "parent",
					Target:   &ofga.Entity{Kind: "document", ID: "doc2"},
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
				// org2#member is editor of folder2
				{
					Object: &ofga.Entity{
						Kind:     "organization",
						ID:       "org2",
						Relation: "member",
					},
					Relation: "editor",
					Target:   &ofga.Entity{Kind: "folder", ID: "folder2"},
				},
				// alice is member of org1 (should access doc1 only)
				{
					Object:   &ofga.Entity{Kind: "user", ID: "alice"},
					Relation: "member",
					Target:   &ofga.Entity{Kind: "organization", ID: "org1"},
				},
				// bob is member of org2 (should access doc2 only)
				{
					Object:   &ofga.Entity{Kind: "user", ID: "bob"},
					Relation: "member",
					Target:   &ofga.Entity{Kind: "organization", ID: "org2"},
				},
				// charlie is member of both org1 and org2 (should access both)
				{
					Object:   &ofga.Entity{Kind: "user", ID: "charlie"},
					Relation: "member",
					Target:   &ofga.Entity{Kind: "organization", ID: "org1"},
				},
				{
					Object:   &ofga.Entity{Kind: "user", ID: "charlie"},
					Relation: "member",
					Target:   &ofga.Entity{Kind: "organization", ID: "org2"},
				},
			},
			query: ofga.Tuple{
				Relation: "viewer",
				Target:   &ofga.Entity{Kind: "document", ID: "doc1"},
			},
			maxDepth: 10,
			expectedUsers: []ofga.Entity{
				{Kind: "user", ID: "alice"},
				{Kind: "user", ID: "charlie"},
			},
			expectError: false,
		},
		{
			name:        "multiple_documents_cross_document_verification",
			description: "Verify querying doc2 returns different users than doc1",
			tuples: []ofga.Tuple{
				// doc1 has parent folder1
				{
					Object:   &ofga.Entity{Kind: "folder", ID: "folder1"},
					Relation: "parent",
					Target:   &ofga.Entity{Kind: "document", ID: "doc1"},
				},
				// doc2 has parent folder2
				{
					Object:   &ofga.Entity{Kind: "folder", ID: "folder2"},
					Relation: "parent",
					Target:   &ofga.Entity{Kind: "document", ID: "doc2"},
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
				// org2#member is editor of folder2
				{
					Object: &ofga.Entity{
						Kind:     "organization",
						ID:       "org2",
						Relation: "member",
					},
					Relation: "editor",
					Target:   &ofga.Entity{Kind: "folder", ID: "folder2"},
				},
				// alice is member of org1 only
				{
					Object:   &ofga.Entity{Kind: "user", ID: "alice"},
					Relation: "member",
					Target:   &ofga.Entity{Kind: "organization", ID: "org1"},
				},
				// bob is member of org2 only
				{
					Object:   &ofga.Entity{Kind: "user", ID: "bob"},
					Relation: "member",
					Target:   &ofga.Entity{Kind: "organization", ID: "org2"},
				},
			},
			query: ofga.Tuple{
				Relation: "viewer",
				Target:   &ofga.Entity{Kind: "document", ID: "doc2"},
			},
			maxDepth: 10,
			expectedUsers: []ofga.Entity{
				{Kind: "user", ID: "bob"},
			},
			expectError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Setup OpenFGA client and store for each test case
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

			// Add relations if any tuples are provided
			if len(test.tuples) > 0 {
				err = ofgaClient.AddRelation(t.Context(), test.tuples...)
				if err != nil {
					t.Fatalf("Failed to add relations: %v", err)
				}
			}

			// Execute the query
			users, err := ofgaClient.FindUsersByRelation(t.Context(), test.query, test.maxDepth)

			// Check error expectation
			if test.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("FindUsersByRelation failed: %v", err)
			}

			// Sort users by ID for consistent comparison
			sortUsersByID := func(entities []ofga.Entity) {
				sort.Slice(entities, func(i, j int) bool {
					return entities[i].ID < entities[j].ID
				})
			}
			sortUsersByID(users)
			sortUsersByID(test.expectedUsers)

			// Compare results
			if len(users) != len(test.expectedUsers) {
				t.Errorf("Expected %d users, got %d\nExpected: %v\nGot: %v",
					len(test.expectedUsers), len(users), test.expectedUsers, users)
				return
			}

			for i, expected := range test.expectedUsers {
				got := users[i]
				if got.Kind != expected.Kind || got.ID != expected.ID || got.Relation != expected.Relation {
					t.Errorf("User at position %d mismatch\nExpected: %+v\nGot: %+v",
						i, expected, got)
				}
			}
		})
	}
}
