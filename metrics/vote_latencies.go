package metrics

import (
	"context"
	"github.com/bft-labs/cometbft-analyzer-backend/types"
	"github.com/bft-labs/cometbft-analyzer-types/pkg/statistics/vote"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"time"
)

// VoteLatencyResult contains both the data and total count for pagination
type VoteLatencyResult struct {
	Data  []*vote.VoteLatency
	Total int
}

func GetVoteLatencies(
	ctx context.Context, coll *mongo.Collection,
	from, to time.Time, page, perPage int, percentile string,
) (*VoteLatencyResult, error) {
	// Convert percentile string to value
	var percentileValue float64
	var percentileKey string
	switch percentile {
	case "p50":
		percentileValue = 0.50
		percentileKey = "p50"
	case "p95":
		percentileValue = 0.95
		percentileKey = "p95"
	case "p99":
		percentileValue = 0.99
		percentileKey = "p99"
	default:
		percentileValue = 0.95 // Default to p95
		percentileKey = "p95"
	}

	// First get percentile threshold
	percentilePipeline := mongo.Pipeline{
		{{"$match", bson.D{
			{"status", string(vote.VoteMsgStatusConfirmed)},
			{"sentTime", bson.D{{"$gte", from}, {"$lte", to}}},
		}}},
		{{"$group", bson.D{
			{"_id", nil},
			{percentileKey, bson.D{{"$percentile", bson.D{
				{"input", "$latency"},
				{"p", bson.A{percentileValue}},
				{"method", "approximate"},
			}}}},
		}}},
	}

	cursor, err := coll.Aggregate(ctx, percentilePipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var percentileResult map[string][]float64
	if cursor.Next(ctx) {
		if err := cursor.Decode(&percentileResult); err != nil {
			return nil, err
		}
	}
	cursor.Close(ctx)

	// If no percentile result, return empty
	thresholdValues, exists := percentileResult[percentileKey]
	if !exists || len(thresholdValues) == 0 {
		return &VoteLatencyResult{Data: []*vote.VoteLatency{}, Total: 0}, nil
	}

	threshold := thresholdValues[0]

	// Create match stage for filtered data
	matchStage := bson.D{{"$match", bson.D{
		{"status", string(vote.VoteMsgStatusConfirmed)},
		{"sentTime", bson.D{{"$gte", from}, {"$lte", to}}},
		{"latency", bson.D{{"$gte", threshold}}},
	}}}

	// Get total count
	countPipeline := mongo.Pipeline{
		matchStage,
		{{"$count", "total"}},
	}

	countCursor, err := coll.Aggregate(ctx, countPipeline)
	if err != nil {
		return nil, err
	}
	defer countCursor.Close(ctx)

	var countResult struct {
		Total int `bson:"total"`
	}
	if countCursor.Next(ctx) {
		if err := countCursor.Decode(&countResult); err != nil {
			return nil, err
		}
	}
	countCursor.Close(ctx)

	// Calculate skip value
	skip := (page - 1) * perPage

	// Get paginated data
	dataPipeline := mongo.Pipeline{
		matchStage,
		{{"$sort", bson.D{{"sentTime", 1}}}}, // Sort by sentTime ascending
		{{"$skip", skip}},
		{{"$limit", perPage}},
	}

	dataCursor, err := coll.Aggregate(ctx, dataPipeline)
	if err != nil {
		return nil, err
	}
	defer dataCursor.Close(ctx)

	var latencies []*vote.VoteLatency
	for dataCursor.Next(ctx) {
		var latency vote.VoteLatency
		if err := dataCursor.Decode(&latency); err != nil {
			return nil, err
		}
		latencies = append(latencies, &latency)
	}
	if err := dataCursor.Err(); err != nil {
		return nil, err
	}

	return &VoteLatencyResult{
		Data:  latencies,
		Total: countResult.Total,
	}, nil
}

// ComputeVoteStatistics returns aggregated statistics grouped by sender, receiver, and vote type
func ComputeVoteStatistics(ctx context.Context, coll *mongo.Collection, from, to time.Time) ([]types.VoteStatisticsResponse, error) {
	pipeline := mongo.Pipeline{
		{{"$match", bson.D{
			{"status", string(vote.VoteMsgStatusConfirmed)},
			{"sentTime", bson.D{{"$gte", from}, {"$lte", to}}},
		}}},
		{{"$group", bson.D{
			{"_id", bson.D{
				{"sender", "$senderPeerId"},
				{"receiver", "$recipientPeerId"},
				{"voteType", "$vote.type"},
			}},
			{"count", bson.D{{"$sum", 1}}},
			{"latencies", bson.D{{"$push", "$latency"}}},
		}}},
		{{"$addFields", bson.D{
			{"p50", bson.D{{"$percentile", bson.D{
				{"input", "$latencies"},
				{"p", bson.A{0.5}},
				{"method", "approximate"},
			}}}},
			{"p90", bson.D{{"$percentile", bson.D{
				{"input", "$latencies"},
				{"p", bson.A{0.9}},
				{"method", "approximate"},
			}}}},
			{"p95", bson.D{{"$percentile", bson.D{
				{"input", "$latencies"},
				{"p", bson.A{0.95}},
				{"method", "approximate"},
			}}}},
			{"p99", bson.D{{"$percentile", bson.D{
				{"input", "$latencies"},
				{"p", bson.A{0.99}},
				{"method", "approximate"},
			}}}},
			{"max", bson.D{{"$max", "$latencies"}}},
		}}},
		{{"$addFields", bson.D{
			{"p95Value", bson.D{{"$arrayElemAt", bson.A{"$p95", 0}}}},
		}}},
		{{"$addFields", bson.D{
			{"spikeThreshold", bson.D{{"$multiply", bson.A{"$p95Value", 2}}}},
		}}},
		{{"$addFields", bson.D{
			{"spikes", bson.D{{"$size", bson.D{{"$filter", bson.D{
				{"input", "$latencies"},
				{"cond", bson.D{{"$gte", bson.A{"$$this", "$spikeThreshold"}}}},
			}}}}}},
		}}},
		{{"$addFields", bson.D{
			{"spikePerc", bson.D{{"$multiply", bson.A{
				bson.D{{"$divide", bson.A{"$spikes", "$count"}}},
				100,
			}}}},
		}}},
		{{"$sort", bson.D{{"_id.sender", 1}, {"_id.receiver", 1}, {"_id.voteType", 1}}}},
	}

	cursor, err := coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []types.VoteStatisticsResponse
	for cursor.Next(ctx) {
		var result struct {
			ID struct {
				Sender   string `bson:"sender"`
				Receiver string `bson:"receiver"`
				VoteType string `bson:"voteType"`
			} `bson:"_id"`
			Count     int64     `bson:"count"`
			P50       []float64 `bson:"p50"`
			P90       []float64 `bson:"p90"`
			P95       []float64 `bson:"p95"`
			P99       []float64 `bson:"p99"`
			Max       int64     `bson:"max"`
			SpikePerc float64   `bson:"spikePerc"`
		}

		if err := cursor.Decode(&result); err != nil {
			return nil, err
		}

		// Convert nanoseconds to milliseconds and extract percentile values
		p50Ms := 0.0
		if len(result.P50) > 0 {
			p50Ms = float64(result.P50[0]) / 1e6
		}

		p90Ms := 0.0
		if len(result.P90) > 0 {
			p90Ms = float64(result.P90[0]) / 1e6
		}

		p95Ms := 0.0
		if len(result.P95) > 0 {
			p95Ms = float64(result.P95[0]) / 1e6
		}

		p99Ms := 0.0
		if len(result.P99) > 0 {
			p99Ms = float64(result.P99[0]) / 1e6
		}

		maxMs := float64(result.Max) / 1e6

		results = append(results, types.VoteStatisticsResponse{
			Sender:    result.ID.Sender,
			Receiver:  result.ID.Receiver,
			VoteType:  result.ID.VoteType,
			Count:     result.Count,
			P50:       p50Ms,
			P90:       p90Ms,
			P95:       p95Ms,
			P99:       p99Ms,
			Max:       maxMs,
			SpikePerc: result.SpikePerc,
		})
	}

	return results, cursor.Err()
}
