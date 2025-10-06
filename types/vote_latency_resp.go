package types

import "time"

// VoteLatencyResponse represents a single row in the latency table.
type VoteLatencyResponse struct {
	Height       uint64    `json:"height"`
	Round        uint64    `json:"round"`
	Type         string    `json:"type"`
	ValidatorIdx uint64    `json:"validatorIndex"`
	Sender       string    `json:"sender"`
	Receiver     string    `json:"receiver"`
	SentTime     time.Time `json:"sentTime"`
	ReceivedTime time.Time `json:"receivedTime"`
	LatencyMs    float64   `json:"latencyMs"`
}

// PaginatedVoteLatencyResponse represents paginated vote latency data
type PaginatedVoteLatencyResponse struct {
	Data       []VoteLatencyResponse `json:"data"`
	Pagination PaginationMeta        `json:"pagination"`
}

// PaginationMeta contains pagination metadata
type PaginationMeta struct {
	Page       int `json:"page"`       // Current page (1-based)
	PerPage    int `json:"perPage"`    // Items per page
	Total      int `json:"total"`      // Total number of items
	TotalPages int `json:"totalPages"` // Total number of pages
}

// VoteStatisticsResponse represents aggregated vote statistics for the table
type VoteStatisticsResponse struct {
	Sender    string  `json:"sender"`
	Receiver  string  `json:"receiver"`
	VoteType  string  `json:"voteType"`
	Count     int64   `json:"count"`
	P50       float64 `json:"p50"`
	P90       float64 `json:"p90"`
	P95       float64 `json:"p95"`
	P99       float64 `json:"p99"`
	Max       float64 `json:"max"`
	SpikePerc float64 `json:"spikePerc"`
}
