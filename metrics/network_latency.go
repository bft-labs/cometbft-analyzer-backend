package metrics

import (
	"context"
	"fmt"
	"github.com/bft-labs/cometbft-analyzer-backend/types"
	"github.com/bft-labs/cometbft-analyzer-types/pkg/statistics/latency"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// GetNetworkLatencyStats retrieves NodePairLatencyStats directly from MongoDB
func GetNetworkLatencyStats(ctx context.Context, coll *mongo.Collection) ([]latency.NodePairLatencyStats, error) {
	// Count documents first
	count, err := coll.CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("error counting documents: %v", err)
	}

	if count == 0 {
		return []latency.NodePairLatencyStats{}, nil
	}

	// Sort by node pair key for consistent ordering
	opts := options.Find().SetSort(bson.D{{"nodePairKey", 1}})

	cursor, err := coll.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, fmt.Errorf("error finding documents: %v", err)
	}
	defer cursor.Close(ctx)

	var stats []latency.NodePairLatencyStats
	if err := cursor.All(ctx, &stats); err != nil {
		return nil, fmt.Errorf("error decoding documents: %v", err)
	}

	return stats, nil
}

// GetNetworkLatencyOverview computes comprehensive network latency statistics
func GetNetworkLatencyOverview(ctx context.Context, coll *mongo.Collection) (*types.NetworkLatencyOverviewResponse, error) {
	// For now, get all documents to test - we can add time filtering later
	filter := bson.M{}

	cursor, err := coll.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	// Data structures to accumulate statistics
	messageTypeStats := make(map[string]struct {
		totalWeightedP95 float64
		totalCount       int
	})
	nodeStats := make(map[string]struct {
		totalWeightedP95 float64
		totalCount       int
	})

	var overallWeightedP95 float64
	var overallCount int

	docCount := 0
	// Process each document
	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		docCount++

		node1Id, _ := doc["node1Id"].(string)
		node2Id, _ := doc["node2Id"].(string)
		fmt.Printf("Processing document %d: node1=%s, node2=%s\n", docCount, node1Id, node2Id)

		// Process messageTypes
		if messageTypes, ok := doc["messageTypes"].(bson.M); ok {
			for msgType, msgData := range messageTypes {
				if msgInfo, ok := msgData.(bson.M); ok {
					if count, ok := msgInfo["count"].(int32); ok {
						if p95Latency, ok := msgInfo["p95LatencyMs"].(int64); ok {
							weightedP95 := float64(p95Latency) * float64(count)

							// Accumulate for message type statistics
							if stat, exists := messageTypeStats[msgType]; exists {
								stat.totalWeightedP95 += weightedP95
								stat.totalCount += int(count)
								messageTypeStats[msgType] = stat
							} else {
								messageTypeStats[msgType] = struct {
									totalWeightedP95 float64
									totalCount       int
								}{weightedP95, int(count)}
							}

							// Accumulate for overall statistics
							overallWeightedP95 += weightedP95
							overallCount += int(count)

							// Accumulate for both nodes
							if stat, exists := nodeStats[node1Id]; exists {
								stat.totalWeightedP95 += weightedP95 / 2 // Split between sender and receiver
								stat.totalCount += int(count) / 2
								nodeStats[node1Id] = stat
							} else {
								nodeStats[node1Id] = struct {
									totalWeightedP95 float64
									totalCount       int
								}{weightedP95 / 2, int(count) / 2}
							}

							if stat, exists := nodeStats[node2Id]; exists {
								stat.totalWeightedP95 += weightedP95 / 2
								stat.totalCount += int(count) / 2
								nodeStats[node2Id] = stat
							} else {
								nodeStats[node2Id] = struct {
									totalWeightedP95 float64
									totalCount       int
								}{weightedP95 / 2, int(count) / 2}
							}
						}
					}
				}
			}
		}
	}

	// Calculate weighted averages and find highest values
	response := &types.NetworkLatencyOverviewResponse{
		MessageTypeLatency:      make(map[string]float64),
		NodeLatencyContribution: make(map[string]float64),
	}

	// Overall weighted average P95 latency
	if overallCount > 0 {
		response.OverallWeightedAvgP95LatencyMs = overallWeightedP95 / float64(overallCount)
	}

	// Message type latency and find highest
	var highestMsgType string
	var highestMsgLatency float64
	for msgType, stat := range messageTypeStats {
		if stat.totalCount > 0 {
			avgLatency := stat.totalWeightedP95 / float64(stat.totalCount)
			response.MessageTypeLatency[msgType] = avgLatency
			if avgLatency > highestMsgLatency {
				highestMsgLatency = avgLatency
				highestMsgType = msgType
			}
		}
	}
	response.MessageTypeWithHighestAvgP95 = types.MessageTypeLatencyInfo{
		MessageType: highestMsgType,
		LatencyMs:   highestMsgLatency,
	}

	// Node latency contribution and find highest
	var highestNodeId string
	var highestNodeLatency float64
	for nodeId, stat := range nodeStats {
		if stat.totalCount > 0 {
			avgLatency := stat.totalWeightedP95 / float64(stat.totalCount)
			response.NodeLatencyContribution[nodeId] = avgLatency
			if avgLatency > highestNodeLatency {
				highestNodeLatency = avgLatency
				highestNodeId = nodeId
			}
		}
	}
	response.NodeWithHighestAvgP95 = types.NodeLatencyInfo{
		NodeId:    highestNodeId,
		LatencyMs: highestNodeLatency,
	}

	return response, nil
}
