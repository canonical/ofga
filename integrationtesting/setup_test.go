// Copyright 2025 Canonical Ltd.
// Licensed under the LGPL license, see LICENSE file for details.

package integration

import (
	_ "embed"
	"encoding/json"
	"os"
	"testing"

	openfga "github.com/openfga/go-sdk"
	"github.com/openfga/go-sdk/client"
)

//go:embed test-model.json
var testModelData []byte

// setupTestClient creates an OpenFGA client, store, and authorization model
func setupTestClient(t *testing.T) (*client.OpenFgaClient, string, string) {
	t.Helper()

	// Create OpenFGA client
	config := &client.ClientConfiguration{
		ApiUrl: "http://localhost:8080",
	}

	fgaClient, err := client.NewSdkClient(config)
	if err != nil {
		t.Fatalf("Failed to create OpenFGA client: %v", err)
	}

	// Create store
	storeResp, err := fgaClient.CreateStore(t.Context()).Body(client.ClientCreateStoreRequest{
		Name: "integration-test-store",
	}).Execute()
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	storeID := storeResp.Id

	var authModel openfga.AuthorizationModel
	if err := json.Unmarshal(testModelData, &authModel); err != nil {
		t.Fatalf("Failed to parse authorization model: %v", err)
	}

	// Set store ID for subsequent operations
	err = fgaClient.SetStoreId(storeID)
	if err != nil {
		t.Fatalf("Failed to set store ID: %v", err)
	}

	// Create authorization model
	modelResp, err := fgaClient.WriteAuthorizationModel(t.Context()).Body(client.ClientWriteAuthorizationModelRequest{
		SchemaVersion:   authModel.SchemaVersion,
		TypeDefinitions: authModel.TypeDefinitions,
	}).Execute()
	if err != nil {
		t.Fatalf("Failed to create authorization model: %v", err)
	}

	modelID := modelResp.AuthorizationModelId
	err = fgaClient.SetAuthorizationModelId(modelID)
	if err != nil {
		t.Fatalf("Failed to set authorization model ID: %v", err)
	}

	t.Logf("Test setup complete - Store ID: %s, Model ID: %s", storeID, modelID)

	return fgaClient, storeID, modelID
}

// TestMain sets up the test environment
func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
