package entity

import (
	"time"
)

// GoodEvent request for ClickHouse
type GoodEvent struct {
	Id          int       `json:"id"`
	ProjectId   int       `json:"projectId"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Priority    int       `json:"priority"`
	Removed     bool      `json:"removed"`
	EventTime   time.Time `json:"createdAt"`
}

// GoodCreateRequest request for create good
type GoodCreateRequest struct {
	Name string `json:"name"`
}

// GoodCreateResponse response for good create request
type GoodCreateResponse struct {
	Id          int       `json:"id"`
	ProjectId   int       `json:"projectId"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Priority    int       `json:"priority"`
	Removed     bool      `json:"removed"`
	CreatedAt   time.Time `json:"createdAt"`
}

// GoodUpdateRequest request for good update
type GoodUpdateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"` // optional field
}

// GoodUpdateResponse response for good update request
type GoodUpdateResponse struct {
	Id          int       `json:"id"`
	ProjectId   int       `json:"projectId"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Priority    int       `json:"priority"`
	Removed     bool      `json:"removed"`
	CreatedAt   time.Time `json:"createdAt"`
}

// GoodRemoveResponse response for good delete request
type GoodRemoveResponse struct {
	Id        int  `json:"id"`
	ProjectId int  `json:"projectId"`
	Removed   bool `json:"removed"`
}

// GoodsListResponse response for list request
type GoodsListResponse struct {
	Meta  MetaForList    `json:"meta"`
	Goods []GoodsForList `json:"goods"`
}

// MetaForList response for list request
type MetaForList struct {
	Total   int `json:"total"`
	Removed int `json:"removed"`
	Limit   int `json:"limit"`
	Offset  int `json:"offset"`
}

// GoodsForList response for list request
type GoodsForList struct {
	Id          int       `json:"id"`
	ProjectId   int       `json:"projectId"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Priority    int       `json:"priority"`
	Removed     bool      `json:"removed"`
	CreatedAt   time.Time `json:"createdAt"`
}

// ReprioritizeRequest request for Reprioritize
type ReprioritizeRequest struct {
	NewPriority int `json:"newPriority"`
}

// ReprioritizeResponse response for reprioritize request
type ReprioritizeResponse struct {
	Id       int `json:"id"`
	Priority int `json:"priority"`
}
