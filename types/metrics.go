package types

// PairLatency represents latency percentiles for a given sender→receiver pair.
type PairLatency struct {
	Sender   string  `json:"sender"`   // Node ID of the sender
	Receiver string  `json:"receiver"` // Node ID of the receiver
	P50Ms    float32 `json:"p50Ms"`    // 50th percentile latency in milliseconds
	P95Ms    float32 `json:"p95Ms"`    // 95th percentile latency in milliseconds
	P99Ms    float32 `json:"p99Ms"`    // 99th percentile latency in milliseconds
}

// BlockLatencyPoint is a single latency measurement record tied to a block height.
type BlockLatencyPoint struct {
	Height    uint64  `json:"height"`    // Block height
	Sender    string  `json:"sender"`    // Node ID of the sender
	Receiver  string  `json:"receiver"`  // Node ID of the receiver
	LatencyMs float32 `json:"latencyMs"` // Measured latency in milliseconds
}

// LatencyHistogramBucket represents a bucket in the latency distribution.
type LatencyHistogramBucket struct {
	Lower float32 `json:"lower"` // Lower bound of the bucket (ms)
	Upper float32 `json:"upper"` // Upper bound of the bucket (ms)
	Count int64   `json:"count"` // Number of samples in this bucket
}

// LatencyJitter holds standard deviation (jitter) info for a sender→receiver pair.
type LatencyJitter struct {
	Sender   string  `json:"sender"`   // Node ID of the sender
	Receiver string  `json:"receiver"` // Node ID of the receiver
	StdDevMs float32 `json:"stdDevMs"` // Standard deviation of latency (ms)
}

// LatencyStats aggregates histogram and jitter facets.
type LatencyStats struct {
	Histogram []LatencyHistogramBucket `json:"histogram"` // Latency distribution buckets
	Jitter    []LatencyJitter          `json:"jitter"`    // Per-pair jitter stats
}

// MessageSuccessRate measures send vs receive counts and delivery ratio.
type MessageSuccessRate struct {
	Height      uint64  `json:"height"`      // Block height
	Sender      string  `json:"sender"`      // Node ID of the sender
	Receiver    string  `json:"receiver"`    // Node ID of the receiver
	SentCount   int64   `json:"sentCount"`   // Total send events
	RecvCount   int64   `json:"recvCount"`   // Total receive events
	SuccessRate float32 `json:"successRate"` // recvCount / sentCount
}

// BlockConsensusLatency captures consensus end-to-end latency per block.
type BlockConsensusLatency struct {
	Height uint64  `json:"height"` // Block height
	P50Ms  float32 `json:"p50Ms"`  // 50th percentile end-to-end latency (ms)
	P95Ms  float32 `json:"p95Ms"`  // 95th percentile end-to-end latency (ms)
}
