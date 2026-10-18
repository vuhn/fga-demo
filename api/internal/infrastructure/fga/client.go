// Package fga implements the authorization gateways on top of the OpenFGA Go SDK.
package fga

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	fga "github.com/openfga/go-sdk"
	fgaclient "github.com/openfga/go-sdk/client"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
)

const (
	maxWriteKeys  = 100
	maxBatchCheck = 50
)

var consistency = fga.CONSISTENCYPREFERENCE_HIGHER_CONSISTENCY

// Client is bound to one store and one authorization model.
type Client struct {
	sdk     *fgaclient.OpenFgaClient
	StoreID string
	ModelID string
}

var _ gateway.AuthzGateway = (*Client)(nil)

func newSDK(apiURL, storeID, modelID string) (*fgaclient.OpenFgaClient, error) {
	return fgaclient.NewSdkClient(&fgaclient.ClientConfiguration{
		ApiUrl:               apiURL,
		StoreId:              storeID,
		AuthorizationModelId: modelID,
	})
}

func NewClient(apiURL, storeID, modelID string) (*Client, error) {
	sdk, err := newSDK(apiURL, storeID, modelID)
	if err != nil {
		return nil, err
	}
	return &Client{sdk: sdk, StoreID: storeID, ModelID: modelID}, nil
}

func (c *Client) Check(ctx context.Context, userID, relation, objectType, objectID string) (bool, error) {
	resp, err := c.sdk.Check(ctx).
		Body(fgaclient.ClientCheckRequest{User: userRef(userID), Relation: relation, Object: obj(objectType, objectID)}).
		Options(fgaclient.ClientCheckOptions{AuthorizationModelId: &c.ModelID, Consistency: &consistency}).
		Execute()
	if err != nil {
		return false, fmt.Errorf("openfga check %s %s %s:%s: %w", userID, relation, objectType, objectID, err)
	}
	return resp.GetAllowed(), nil
}

func (c *Client) BatchCheck(ctx context.Context, checks []entity.CheckRequest) ([]bool, error) {
	out := make([]bool, len(checks))
	for start := 0; start < len(checks); start += maxBatchCheck {
		end := min(start+maxBatchCheck, len(checks))
		items := make([]fgaclient.ClientBatchCheckItem, 0, end-start)
		for i := start; i < end; i++ {
			ch := checks[i]
			items = append(items, fgaclient.ClientBatchCheckItem{
				User:          userRef(ch.UserID),
				Relation:      ch.Relation,
				Object:        obj(ch.ObjectType, ch.ObjectID),
				CorrelationId: strconv.Itoa(i),
			})
		}
		size := int32(maxBatchCheck)
		resp, err := c.sdk.BatchCheck(ctx).
			Body(fgaclient.ClientBatchCheckRequest{Checks: items}).
			Options(fgaclient.BatchCheckOptions{AuthorizationModelId: &c.ModelID, Consistency: &consistency, MaxBatchSize: &size}).
			Execute()
		if err != nil {
			return nil, fmt.Errorf("openfga batch check: %w", err)
		}
		for id, res := range resp.GetResult() {
			i, err := strconv.Atoi(id)
			if err != nil || i < start || i >= end {
				return nil, fmt.Errorf("openfga batch check: unexpected correlation id %q", id)
			}
			if res.Error != nil {
				return nil, fmt.Errorf("openfga batch check item %d: %s", i, res.Error.GetMessage())
			}
			out[i] = res.GetAllowed()
		}
	}
	return out, nil
}

func (c *Client) ListObjectIDs(ctx context.Context, userID, relation, objectType string) ([]string, error) {
	resp, err := c.sdk.ListObjects(ctx).
		Body(fgaclient.ClientListObjectsRequest{User: userRef(userID), Relation: relation, Type: objectType}).
		Options(fgaclient.ClientListObjectsOptions{AuthorizationModelId: &c.ModelID, Consistency: &consistency}).
		Execute()
	if err != nil {
		return nil, fmt.Errorf("openfga list objects %s %s %s: %w", userID, relation, objectType, err)
	}
	ids := make([]string, 0, len(resp.GetObjects()))
	for _, o := range resp.GetObjects() {
		ids = append(ids, strings.TrimPrefix(o, objectType+":"))
	}
	return ids, nil
}

func (c *Client) ReadTuples(ctx context.Context, objectType, objectID string) ([]entity.Tuple, error) {
	o := obj(objectType, objectID)
	var out []entity.Tuple
	var token string
	for {
		opts := fgaclient.ClientReadOptions{}
		if token != "" {
			opts.ContinuationToken = &token
		}
		resp, err := c.sdk.Read(ctx).Body(fgaclient.ClientReadRequest{Object: &o}).Options(opts).Execute()
		if err != nil {
			return nil, fmt.Errorf("openfga read %s: %w", o, err)
		}
		for _, t := range resp.GetTuples() {
			k := t.GetKey()
			out = append(out, entity.Tuple{User: k.GetUser(), Relation: k.GetRelation(), Object: k.GetObject()})
		}
		token = resp.GetContinuationToken()
		if token == "" {
			return out, nil
		}
	}
}

func (c *Client) Write(ctx context.Context, writes, deletes []entity.Tuple) error {
	if len(writes)+len(deletes) == 0 {
		return nil
	}
	if len(writes)+len(deletes) > maxWriteKeys {
		return fmt.Errorf("openfga write: %d tuple keys exceeds the atomic limit of %d", len(writes)+len(deletes), maxWriteKeys)
	}
	body := fgaclient.ClientWriteRequest{}
	for _, t := range writes {
		body.Writes = append(body.Writes, fgaclient.ClientTupleKey{User: t.User, Relation: t.Relation, Object: t.Object})
	}
	for _, t := range deletes {
		body.Deletes = append(body.Deletes, fgaclient.ClientTupleKeyWithoutCondition{User: t.User, Relation: t.Relation, Object: t.Object})
	}
	_, err := c.sdk.Write(ctx).Body(body).Options(fgaclient.ClientWriteOptions{
		AuthorizationModelId: &c.ModelID,
		Conflict: fgaclient.ClientWriteConflictOptions{
			OnDuplicateWrites: fgaclient.CLIENT_WRITE_REQUEST_ON_DUPLICATE_WRITES_IGNORE,
			OnMissingDeletes:  fgaclient.CLIENT_WRITE_REQUEST_ON_MISSING_DELETES_IGNORE,
		},
	}).Execute()
	if err != nil {
		return fmt.Errorf("openfga write: %w", err)
	}
	return nil
}
