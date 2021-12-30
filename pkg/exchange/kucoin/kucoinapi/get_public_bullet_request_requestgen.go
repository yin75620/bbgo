// Code generated by "requestgen -type GetPublicBulletRequest -method POST -url /api/v1/bullet-public -responseType .APIResponse -responseDataField Data -responseDataType .Bullet"; DO NOT EDIT.

package kucoinapi

import (
	"context"
	"encoding/json"
	"net/url"
)

func (g *GetPublicBulletRequest) Do(ctx context.Context) (*Bullet, error) {

	// no body params
	var params interface{}
	query := url.Values{}

	req, err := g.client.NewRequest(ctx, "POST", "/api/v1/bullet-public", query, params)
	if err != nil {
		return nil, err
	}

	response, err := g.client.SendRequest(req)
	if err != nil {
		return nil, err
	}

	var apiResponse APIResponse
	if err := response.DecodeJSON(&apiResponse); err != nil {
		return nil, err
	}
	var data Bullet
	if err := json.Unmarshal(apiResponse.Data, &data); err != nil {
		return nil, err
	}
	return &data, nil
}