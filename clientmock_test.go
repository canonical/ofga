package ofga_test

import (
	"context"
	"go/types"
	"net/http"

	openfga "github.com/openfga/go-sdk"
)

type (
	WriteResponse           map[string]interface{}
	DeleteStoreResponse     types.Nil
	WriteAssertionsResponse types.Nil
)

type ResponseType interface {
	openfga.CreateStoreResponse | openfga.CheckResponse | DeleteStoreResponse | openfga.ExpandResponse | openfga.GetStoreResponse |
		openfga.ListStoresResponse | openfga.ListObjectsResponse | openfga.ReadResponse | openfga.ReadAssertionsResponse |
		openfga.ReadAuthorizationModelResponse | openfga.ReadAuthorizationModelsResponse | openfga.ReadChangesResponse |
		WriteResponse | WriteAssertionsResponse | openfga.WriteAuthorizationModelResponse
}

type MockResponse[RT ResponseType] struct {
	resp     RT
	httpResp *http.Response
	err      error
}

type MockOpenFgaApi struct {
	checkResp           MockResponse[openfga.CheckResponse]
	createStoreResp     MockResponse[openfga.CreateStoreResponse]
	deleteStoreResp     MockResponse[DeleteStoreResponse]
	expandResp          MockResponse[openfga.ExpandResponse]
	getStoreResp        MockResponse[openfga.GetStoreResponse]
	listStoreResp       MockResponse[openfga.ListStoresResponse]
	listObjectsResp     MockResponse[openfga.ListObjectsResponse]
	readResp            MockResponse[openfga.ReadResponse]
	readAssertionsResp  MockResponse[openfga.ReadAssertionsResponse]
	readAuthModelResp   MockResponse[openfga.ReadAuthorizationModelResponse]
	readAuthModelsResp  MockResponse[openfga.ReadAuthorizationModelsResponse]
	readChangesResp     MockResponse[openfga.ReadChangesResponse]
	writeResp           MockResponse[WriteResponse]
	writeAssertionsResp MockResponse[WriteAssertionsResponse]
	writeAuthModelResp  MockResponse[openfga.WriteAuthorizationModelResponse]
}

func (m *MockOpenFgaApi) Check(context.Context) openfga.ApiCheckRequest {
	return openfga.ApiCheckRequest{ApiService: m}
}

func (m *MockOpenFgaApi) CheckExecute(openfga.ApiCheckRequest) (openfga.CheckResponse, *http.Response, error) {
	return m.checkResp.resp, m.checkResp.httpResp, m.checkResp.err
}

func (m *MockOpenFgaApi) CreateStore(context.Context) openfga.ApiCreateStoreRequest {
	return openfga.ApiCreateStoreRequest{ApiService: m}
}

func (m *MockOpenFgaApi) CreateStoreExecute(openfga.ApiCreateStoreRequest) (openfga.CreateStoreResponse, *http.Response, error) {
	return m.createStoreResp.resp, m.createStoreResp.httpResp, m.createStoreResp.err
}

func (m *MockOpenFgaApi) DeleteStore(context.Context) openfga.ApiDeleteStoreRequest {
	return openfga.ApiDeleteStoreRequest{ApiService: m}
}

func (m *MockOpenFgaApi) DeleteStoreExecute(openfga.ApiDeleteStoreRequest) (*http.Response, error) {
	return m.deleteStoreResp.httpResp, m.deleteStoreResp.err
}

func (m *MockOpenFgaApi) Expand(context.Context) openfga.ApiExpandRequest {
	return openfga.ApiExpandRequest{ApiService: m}
}

func (m *MockOpenFgaApi) ExpandExecute(openfga.ApiExpandRequest) (openfga.ExpandResponse, *http.Response, error) {
	return m.expandResp.resp, m.expandResp.httpResp, m.expandResp.err
}

func (m *MockOpenFgaApi) GetStore(context.Context) openfga.ApiGetStoreRequest {
	return openfga.ApiGetStoreRequest{ApiService: m}
}

func (m *MockOpenFgaApi) GetStoreExecute(openfga.ApiGetStoreRequest) (openfga.GetStoreResponse, *http.Response, error) {
	return m.getStoreResp.resp, m.getStoreResp.httpResp, m.getStoreResp.err
}

func (m *MockOpenFgaApi) ListObjects(context.Context) openfga.ApiListObjectsRequest {
	return openfga.ApiListObjectsRequest{ApiService: m}
}

func (m *MockOpenFgaApi) ListObjectsExecute(openfga.ApiListObjectsRequest) (openfga.ListObjectsResponse, *http.Response, error) {
	return m.listObjectsResp.resp, m.listObjectsResp.httpResp, m.listObjectsResp.err
}

func (m *MockOpenFgaApi) ListStores(context.Context) openfga.ApiListStoresRequest {
	return openfga.ApiListStoresRequest{ApiService: m}
}

func (m *MockOpenFgaApi) ListStoresExecute(openfga.ApiListStoresRequest) (openfga.ListStoresResponse, *http.Response, error) {
	return m.listStoreResp.resp, m.listStoreResp.httpResp, m.listStoreResp.err
}

func (m *MockOpenFgaApi) Read(context.Context) openfga.ApiReadRequest {
	return openfga.ApiReadRequest{ApiService: m}
}

func (m *MockOpenFgaApi) ReadExecute(openfga.ApiReadRequest) (openfga.ReadResponse, *http.Response, error) {
	return m.readResp.resp, m.readResp.httpResp, m.readResp.err
}

func (m *MockOpenFgaApi) ReadAssertions(context.Context, string) openfga.ApiReadAssertionsRequest {
	return openfga.ApiReadAssertionsRequest{ApiService: m}
}

func (m *MockOpenFgaApi) ReadAssertionsExecute(openfga.ApiReadAssertionsRequest) (openfga.ReadAssertionsResponse, *http.Response, error) {
	return m.readAssertionsResp.resp, m.readAssertionsResp.httpResp, m.readAssertionsResp.err
}

func (m *MockOpenFgaApi) ReadAuthorizationModel(context.Context, string) openfga.ApiReadAuthorizationModelRequest {
	return openfga.ApiReadAuthorizationModelRequest{ApiService: m}
}

func (m *MockOpenFgaApi) ReadAuthorizationModelExecute(openfga.ApiReadAuthorizationModelRequest) (openfga.ReadAuthorizationModelResponse, *http.Response, error) {
	return m.readAuthModelResp.resp, m.readAuthModelResp.httpResp, m.readAuthModelResp.err
}

func (m *MockOpenFgaApi) ReadAuthorizationModels(context.Context) openfga.ApiReadAuthorizationModelsRequest {
	return openfga.ApiReadAuthorizationModelsRequest{ApiService: m}
}

func (m *MockOpenFgaApi) ReadAuthorizationModelsExecute(openfga.ApiReadAuthorizationModelsRequest) (openfga.ReadAuthorizationModelsResponse, *http.Response, error) {
	return m.readAuthModelsResp.resp, m.readAuthModelsResp.httpResp, m.readAuthModelsResp.err
}

func (m *MockOpenFgaApi) ReadChanges(context.Context) openfga.ApiReadChangesRequest {
	return openfga.ApiReadChangesRequest{ApiService: m}
}

func (m *MockOpenFgaApi) ReadChangesExecute(openfga.ApiReadChangesRequest) (openfga.ReadChangesResponse, *http.Response, error) {
	return m.readChangesResp.resp, m.readChangesResp.httpResp, m.readChangesResp.err
}

func (m *MockOpenFgaApi) Write(context.Context) openfga.ApiWriteRequest {
	return openfga.ApiWriteRequest{ApiService: m}
}

func (m *MockOpenFgaApi) WriteExecute(openfga.ApiWriteRequest) (map[string]interface{}, *http.Response, error) {
	return m.writeResp.resp, m.writeResp.httpResp, m.writeResp.err
}

func (m *MockOpenFgaApi) WriteAssertions(context.Context, string) openfga.ApiWriteAssertionsRequest {
	return openfga.ApiWriteAssertionsRequest{ApiService: m}
}

func (m *MockOpenFgaApi) WriteAssertionsExecute(openfga.ApiWriteAssertionsRequest) (*http.Response, error) {
	return m.writeAssertionsResp.httpResp, m.writeAssertionsResp.err
}

func (m *MockOpenFgaApi) WriteAuthorizationModel(context.Context) openfga.ApiWriteAuthorizationModelRequest {
	return openfga.ApiWriteAuthorizationModelRequest{ApiService: m}
}

func (m *MockOpenFgaApi) WriteAuthorizationModelExecute(openfga.ApiWriteAuthorizationModelRequest) (openfga.WriteAuthorizationModelResponse, *http.Response, error) {
	return m.writeAuthModelResp.resp, m.writeAuthModelResp.httpResp, m.writeAuthModelResp.err
}
