package openfga

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	fga "github.com/openfga/go-sdk"
	fgaclient "github.com/openfga/go-sdk/client"
	"github.com/openfga/language/pkg/go/transformer"
)

// Reset deletes every store with the given name, creates a fresh one, and
// writes the model at modelPath. The seed uses it so the demo always starts
// from a known state.
func Reset(ctx context.Context, apiURL, storeName, modelPath string) (*Client, error) {
	admin, err := newSDK(apiURL, "", "")
	if err != nil {
		return nil, err
	}
	stores, err := listStores(ctx, admin, storeName)
	if err != nil {
		return nil, err
	}
	for _, s := range stores {
		if _, err := admin.DeleteStore(ctx).Options(fgaclient.ClientDeleteStoreOptions{StoreId: fga.PtrString(s.GetId())}).Execute(); err != nil {
			return nil, fmt.Errorf("delete store %s: %w", s.GetId(), err)
		}
	}
	created, err := admin.CreateStore(ctx).Body(fgaclient.ClientCreateStoreRequest{Name: storeName}).Execute()
	if err != nil {
		return nil, fmt.Errorf("create store: %w", err)
	}
	modelID, err := WriteModel(ctx, apiURL, created.GetId(), modelPath)
	if err != nil {
		return nil, err
	}
	return NewClient(apiURL, created.GetId(), modelID)
}

// Open finds the store by name and binds to its newest model.
func Open(ctx context.Context, apiURL, storeName string) (*Client, error) {
	admin, err := newSDK(apiURL, "", "")
	if err != nil {
		return nil, err
	}
	stores, err := listStores(ctx, admin, storeName)
	if err != nil {
		return nil, err
	}
	if len(stores) == 0 {
		return nil, fmt.Errorf("openfga store %q not found: run `go run ./cmd/seed` first", storeName)
	}
	storeID := stores[0].GetId()
	resp, err := admin.ReadAuthorizationModels(ctx).Options(fgaclient.ClientReadAuthorizationModelsOptions{
		StoreId:  &storeID,
		PageSize: fga.PtrInt32(1),
	}).Execute()
	if err != nil {
		return nil, fmt.Errorf("read authorization models: %w", err)
	}
	if len(resp.GetAuthorizationModels()) == 0 {
		return nil, fmt.Errorf("openfga store %q has no model: run `go run ./cmd/seed` first", storeName)
	}
	return NewClient(apiURL, storeID, resp.GetAuthorizationModels()[0].GetId())
}

// WriteModel transforms the DSL file and writes it to the store.
func WriteModel(ctx context.Context, apiURL, storeID, modelPath string) (string, error) {
	dsl, err := os.ReadFile(modelPath)
	if err != nil {
		return "", fmt.Errorf("read model: %w", err)
	}
	raw, err := transformer.TransformDSLToJSON(string(dsl))
	if err != nil {
		return "", fmt.Errorf("parse model: %w", err)
	}
	var req fgaclient.ClientWriteAuthorizationModelRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		return "", fmt.Errorf("decode model: %w", err)
	}
	sdk, err := newSDK(apiURL, storeID, "")
	if err != nil {
		return "", err
	}
	resp, err := sdk.WriteAuthorizationModel(ctx).Body(req).Execute()
	if err != nil {
		return "", fmt.Errorf("write model: %w", err)
	}
	return resp.GetAuthorizationModelId(), nil
}

// DeleteStore removes the bound store. Tests use it for cleanup.
func (c *Client) DeleteStore(ctx context.Context) error {
	_, err := c.sdk.DeleteStore(ctx).Execute()
	return err
}

func listStores(ctx context.Context, sdk *fgaclient.OpenFgaClient, name string) ([]fga.Store, error) {
	var out []fga.Store
	var token string
	for {
		opts := fgaclient.ClientListStoresOptions{Name: &name}
		if token != "" {
			opts.ContinuationToken = &token
		}
		resp, err := sdk.ListStores(ctx).Options(opts).Execute()
		if err != nil {
			return nil, fmt.Errorf("list stores: %w", err)
		}
		for _, s := range resp.GetStores() {
			if s.GetName() == name {
				out = append(out, s)
			}
		}
		token = resp.GetContinuationToken()
		if token == "" {
			return out, nil
		}
	}
}
