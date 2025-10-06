package metrics

import (
	"context"
	"github.com/bft-labs/cometbft-analyzer-backend/types"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
)

// 1. Pair-wise latency percentiles (p50, p95, p99) per sender→receiver
func ComputePairwiseLatencyPercentiles(
	ctx context.Context, coll *mongo.Collection,
	from, to time.Time,
) ([]types.PairLatency, error) {
	pipeline := mongo.Pipeline{
		{{"$match", bson.D{
			{"sentTime", bson.D{
				{"$gte", from},
				{"$lte", to},
			}},
			{"status", "confirmed"},
		}}},
		{{"$addFields", bson.D{
			{"latencyMs", bson.D{{"$divide", bson.A{"$latency", 1000000}}}}, // convert nanoseconds to milliseconds
		}}},
		{{"$group", bson.D{
			{"_id", bson.D{
				{"sender", "$senderPeerId"},
				{"receiver", "$recipientPeerId"},
			}},
			{"p50", bson.D{{"$percentile", bson.D{
				{"input", "$latencyMs"},
				{"p", bson.A{0.50}},
				{"method", "approximate"},
			}}}},
			{"p95", bson.D{{"$percentile", bson.D{
				{"input", "$latencyMs"},
				{"p", bson.A{0.95}},
				{"method", "approximate"},
			}}}},
			{"p99", bson.D{{"$percentile", bson.D{
				{"input", "$latencyMs"},
				{"p", bson.A{0.99}},
				{"method", "approximate"},
			}}}},
		}}},
		{{"$project", bson.D{
			{"_id", 0},
			{"sender", "$_id.sender"},
			{"receiver", "$_id.receiver"},
			{"p50Ms", bson.D{{"$arrayElemAt", bson.A{"$p50", 0}}}},
			{"p95Ms", bson.D{{"$arrayElemAt", bson.A{"$p95", 0}}}},
			{"p99Ms", bson.D{{"$arrayElemAt", bson.A{"$p99", 0}}}},
		}}},
	}

	opts := options.Aggregate().SetAllowDiskUse(true)
	cur, err := coll.Aggregate(ctx, pipeline, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var rawResults []bson.M
	if err := cur.All(ctx, &rawResults); err != nil {
		return nil, err
	}

	var out []types.PairLatency
	for _, doc := range rawResults {
		out = append(out, types.PairLatency{
			Sender:   doc["sender"].(string),
			Receiver: doc["receiver"].(string),
			P50Ms:    float32(doc["p50Ms"].(float64)),
			P95Ms:    float32(doc["p95Ms"].(float64)),
			P99Ms:    float32(doc["p99Ms"].(float64)),
		})
	}
	return out, nil
}

// 2. Block-based time-series: each send→receive latency per height, sender, receiver
func ComputeBlockLatencyTimeSeries(
	ctx context.Context, coll *mongo.Collection,
	from, to time.Time,
) ([]types.BlockLatencyPoint, error) {
	matchTime := bson.D{{"$match", bson.D{
		{"timestamp", bson.D{
			{"$gte", from},
			{"$lte", to},
		}},
		{"type", "sendVote"},
	}}}
	pipeline := mongo.Pipeline{
		matchTime,
		{{"$match", bson.D{{"type", "sendVote"}}}},
		{{"$lookup", bson.D{
			{"from", "events"},
			{"let", bson.D{
				{"h", "$vote.height"},
				{"r", "$vote.round"},
				{"vIdx", "$vote.validatorIndex"},
				{"sendTs", "$timestamp"},
				{"snd", "$nodeId"},
				{"recPe", "$recipientPeerId"},
			}},
			{"pipeline", mongo.Pipeline{
				{{"$match", bson.D{{"$expr", bson.D{{"$and", bson.A{
					bson.D{{"$eq", bson.A{"$type", "receiveVote"}}},
					bson.D{{"$eq", bson.A{"$vote.height", "$$h"}}},
					bson.D{{"$eq", bson.A{"$vote.round", "$$r"}}},
					bson.D{{"$eq", bson.A{"$vote.validatorIndex", "$$vIdx"}}},
					bson.D{{"$eq", bson.A{"$sourcePeerId", "$$snd"}}},
					bson.D{{"$eq", bson.A{"$nodeId", "$$recPe"}}},
				}}}}}}},
				{{"$project", bson.D{
					{"height", "$$h"},
					{"sender", "$$snd"},
					{"receiver", "$$recPe"},
					{"latencyMs", bson.D{{"$subtract", bson.A{"$timestamp", "$$sendTs"}}}},
				}}},
			}},
			{"as", "recvDocs"},
		}}},
		{{"$unwind", "$recvDocs"}},
		{{"$replaceRoot", bson.D{{"newRoot", "$recvDocs"}}}},
		{{"$sort", bson.D{{"height", 1}}}},
	}

	opts := options.Aggregate().SetAllowDiskUse(true)
	cur, err := coll.Aggregate(ctx, pipeline, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var series []types.BlockLatencyPoint
	if err := cur.All(ctx, &series); err != nil {
		return nil, err
	}
	return series, nil
}

// 3. Latency distribution (histogram via bucketAuto) & jitter (stdDev) per pair
func ComputeLatencyStats(
	ctx context.Context, coll *mongo.Collection,
	from, to time.Time,
) (*types.LatencyStats, error) {
	matchTime := bson.D{{"$match", bson.D{
		{"timestamp", bson.D{
			{"$gte", from},
			{"$lte", to},
		}},
		{"type", "sendVote"},
	}}}
	// facet into histogram and stddev facets
	pipeline := mongo.Pipeline{
		matchTime,
		{{"$match", bson.D{{"type", "sendVote"}}}},
		{{"$lookup", bson.D{
			{"from", "events"},
			{"let", bson.D{
				{"h", "$vote.height"},
				{"r", "$vote.round"},
				{"vIdx", "$vote.validatorIndex"},
				{"sendTs", "$timestamp"},
				{"snd", "$nodeId"},
				{"recPe", "$recipientPeerId"},
			}},
			{"pipeline", mongo.Pipeline{
				{{"$match", bson.D{{"$expr", bson.D{{"$and", bson.A{
					bson.D{{"$eq", bson.A{"$type", "receiveVote"}}},
					bson.D{{"$eq", bson.A{"$vote.height", "$$h"}}},
					bson.D{{"$eq", bson.A{"$vote.round", "$$r"}}},
					bson.D{{"$eq", bson.A{"$vote.validatorIndex", "$$vIdx"}}},
					bson.D{{"$eq", bson.A{"$sourcePeerId", "$$snd"}}},
					bson.D{{"$eq", bson.A{"$nodeId", "$$recPe"}}},
				}}}}}}},
				{{"$project", bson.D{
					{"sender", "$$snd"},
					{"receiver", "$$recPe"},
					{"latencyMs", bson.D{{"$subtract", bson.A{"$timestamp", "$$sendTs"}}}},
				}}},
			}},
			{"as", "r"},
		}}},
		{{"$unwind", "$r"}},
		{{"$replaceRoot", bson.D{{"newRoot", "$r"}}}},

		// now facet
		{{"$facet", bson.D{
			{"jitter", bson.A{
				bson.D{{"$group", bson.D{
					{"_id", bson.D{{"sender", "$sender"}, {"receiver", "$receiver"}}},
					{"stdDevMs", bson.D{{"$stdDevSamp", "$latencyMs"}}},
				}}},
				bson.D{{"$project", bson.D{
					{"_id", 0},
					{"sender", "$_id.sender"},
					{"receiver", "$_id.receiver"},
					{"stdDevMs", 1},
				}}},
			}},
			{"histogram", bson.A{
				bson.D{{"$bucketAuto", bson.D{
					{"groupBy", "$latencyMs"},
					{"buckets", 10},
				}}},
			}},
		}}},
	}

	opts := options.Aggregate().SetAllowDiskUse(true)
	cur, err := coll.Aggregate(ctx, pipeline, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var faceted []struct {
		Jitter    []types.LatencyJitter          `bson:"jitter"`
		Histogram []types.LatencyHistogramBucket `bson:"histogram"`
	}
	if err = cur.All(ctx, &faceted); err != nil {
		return nil, err
	}
	if len(faceted) == 0 {
		return nil, nil // no data
	}
	return &types.LatencyStats{
		Histogram: faceted[0].Histogram,
		Jitter:    faceted[0].Jitter,
	}, nil
}

// 4. Message success & loss rate per block, per pair
func ComputeMessageSuccessRate(
	ctx context.Context, coll *mongo.Collection,
	from, to time.Time,
) ([]types.MessageSuccessRate, error) {
	matchTime := bson.D{{"$match", bson.D{
		{"timestamp", bson.D{
			{"$gte", from},
			{"$lte", to},
		}},
		{"type", "sendVote"},
	}}}
	pipeline := mongo.Pipeline{
		matchTime,
		{{"$match", bson.D{{"type", bson.D{{"$in", bson.A{"sendVote", "receiveVote"}}}}}}},
		{{"$project", bson.D{
			{"height", "$vote.height"},
			{"pair", bson.D{{"$cond", bson.A{
				bson.D{{"$eq", bson.A{"$type", "sendVote"}}}, bson.A{"$nodeId", "$recipientPeerId"},
				bson.A{"$sourcePeerId", "$nodeId"},
			}}}},
			{"sent", bson.D{{"$cond", bson.A{
				bson.D{{"$eq", bson.A{"$type", "sendVote"}}}, 1, 0,
			}}}},
			{"recv", bson.D{{"$cond", bson.A{
				bson.D{{"$eq", bson.A{"$type", "receiveVote"}}}, 1, 0,
			}}}},
		}}},
		{{"$group", bson.D{
			{"_id", bson.D{
				{"height", "$height"},
				{"sender", bson.D{{"$arrayElemAt", bson.A{"$pair", 0}}}},
				{"receiver", bson.D{{"$arrayElemAt", bson.A{"$pair", 1}}}},
			}},
			{"sentCnt", bson.D{{"$sum", "$sent"}}},
			{"recvCnt", bson.D{{"$sum", "$recv"}}},
		}}},
		{{"$project", bson.D{
			{"_id", 0},
			{"height", "$_id.height"},
			{"sender", "$_id.sender"},
			{"receiver", "$_id.receiver"},
			{"sentCnt", 1},
			{"recvCnt", 1},
			{"successRate", bson.D{{"$cond", bson.A{
				bson.D{{"$eq", bson.A{"$sentCnt", 0}}}, 0,
				bson.D{{"$divide", bson.A{"$recvCnt", "$sentCnt"}}},
			}}}},
		}}},
		{{"$sort", bson.D{{"height", 1}}}},
	}

	opts := options.Aggregate().SetAllowDiskUse(true)
	cur, err := coll.Aggregate(ctx, pipeline, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var rates []types.MessageSuccessRate
	if err := cur.All(ctx, &rates); err != nil {
		return nil, err
	}
	return rates, nil
}

// 5. Block end-to-end consensus latency per height (EnteringNewRound → ReceivedCompleteProposalBlock)
func ComputeBlockEndToEndLatencyByHeight(
	ctx context.Context, coll *mongo.Collection,
	from, to time.Time,
) ([]types.BlockConsensusLatency, error) {
	matchTime := bson.D{{"$match", bson.D{
		{"timestamp", bson.D{
			{"$gte", from},
			{"$lte", to},
		}},
		{"type", "sendVote"},
	}}}
	pipeline := mongo.Pipeline{
		matchTime,
		{{"$match", bson.D{{"type", "enteringNewRound"}}}},
		{{"$lookup", bson.D{
			{"from", "events"},
			{"let", bson.D{
				{"h", "$height"},
				{"startTs", "$timestamp"},
			}},
			{"pipeline", mongo.Pipeline{
				{{"$match", bson.D{{"$expr", bson.D{{"$and", bson.A{
					bson.D{{"$eq", bson.A{"$type", "receivedCompleteProposalBlock"}}},
					bson.D{{"$eq", bson.A{"$height", "$$h"}}},
				}}}}}}},
				{{"$project", bson.D{
					{"height", "$$h"},
					{"latencyMs", bson.D{{"$subtract", bson.A{"$timestamp", "$$startTs"}}}},
				}}},
			}},
			{"as", "latencies"},
		}}},
		{{"$unwind", "$latencies"}},

		// group by block height
		{{"$group", bson.D{
			{"_id", "$latencies.height"},
			{"p50Ms", bson.D{{"$percentile", bson.D{
				{"input", "$latencies.latencyMs"}, {"p", bson.A{0.50}}, {"method", "approximate"},
			}}}},
			{"p95Ms", bson.D{{"$percentile", bson.D{
				{"input", "$latencies.latencyMs"}, {"p", bson.A{0.95}}, {"method", "approximate"},
			}}}},
		}}},
		{{"$project", bson.D{
			{"_id", 0},
			{"height", "$_id"},
			{"p50Ms", 1},
			{"p95Ms", 1},
		}}},
		{{"$sort", bson.D{{"height", 1}}}},
	}

	opts := options.Aggregate().SetAllowDiskUse(true)
	cur, err := coll.Aggregate(ctx, pipeline, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var latencies []types.BlockConsensusLatency
	if err := cur.All(ctx, &latencies); err != nil {
		return nil, err
	}
	return latencies, nil
}
