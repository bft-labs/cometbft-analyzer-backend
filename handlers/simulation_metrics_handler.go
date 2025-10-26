package handlers

import (
	"context"
	"github.com/bft-labs/cometbft-analyzer-backend/types"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"net/http"
)

// Helper function to validate simulation and get database connection
func validateSimulationAndGetDB(c *gin.Context, client *mongo.Client, simulationsColl *mongo.Collection, collectionName string) (*mongo.Collection, bool) {
	// Get simulation ID from path parameter
	simulationID := c.Param("id")
	objectID, err := primitive.ObjectIDFromHex(simulationID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid simulation ID"})
		return nil, false
	}

	// Get simulation to verify it exists
	var simulation types.Simulation
	err = simulationsColl.FindOne(context.Background(), bson.M{"_id": objectID}).Decode(&simulation)
	if err == mongo.ErrNoDocuments {
		c.JSON(http.StatusNotFound, gin.H{"error": "Simulation not found"})
		return nil, false
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return nil, false
	}

	// Connect to simulation-specific database
	databaseName := simulationID
	coll := client.Database(databaseName).Collection(collectionName)

	return coll, true
}

// GetSimulationVoteLatenciesHandler returns paginated vote latencies for a specific simulation
func GetSimulationVoteLatenciesHandler(client *mongo.Client, simulationsColl *mongo.Collection) gin.HandlerFunc {
	return func(c *gin.Context) {
		if coll, ok := validateSimulationAndGetDB(c, client, simulationsColl, "vote_latencies"); ok {
			handler := GetVoteLatenciesHandler(coll)
			handler(c)
		}
	}
}

// GetSimulationPairLatencyHandler returns sender→receiver latency percentiles for a specific simulation
func GetSimulationPairLatencyHandler(client *mongo.Client, simulationsColl *mongo.Collection) gin.HandlerFunc {
	return func(c *gin.Context) {
		if coll, ok := validateSimulationAndGetDB(c, client, simulationsColl, "vote_latencies"); ok {
			handler := GetPairLatencyHandler(coll)
			handler(c)
		}
	}
}

// GetSimulationBlockLatencyTimeSeriesHandler returns per-block latency time-series for a specific simulation
func GetSimulationBlockLatencyTimeSeriesHandler(client *mongo.Client, simulationsColl *mongo.Collection) gin.HandlerFunc {
	return func(c *gin.Context) {
		if coll, ok := validateSimulationAndGetDB(c, client, simulationsColl, "tracer_events"); ok {
			handler := GetBlockLatencyTimeSeriesHandler(coll)
			handler(c)
		}
	}
}

// GetSimulationLatencyStatsHandler returns histogram and jitter stats for a specific simulation
func GetSimulationLatencyStatsHandler(client *mongo.Client, simulationsColl *mongo.Collection) gin.HandlerFunc {
	return func(c *gin.Context) {
		if coll, ok := validateSimulationAndGetDB(c, client, simulationsColl, "tracer_events"); ok {
			handler := GetLatencyStatsHandler(coll)
			handler(c)
		}
	}
}

// GetSimulationMessageSuccessRateHandler returns send vs receive counts and delivery ratio for a specific simulation
func GetSimulationMessageSuccessRateHandler(client *mongo.Client, simulationsColl *mongo.Collection) gin.HandlerFunc {
	return func(c *gin.Context) {
		if coll, ok := validateSimulationAndGetDB(c, client, simulationsColl, "tracer_events"); ok {
			handler := GetMessageSuccessRateHandler(coll)
			handler(c)
		}
	}
}

// GetSimulationBlockEndToEndLatencyHandler returns end-to-end consensus latency per block height for a specific simulation
func GetSimulationBlockEndToEndLatencyHandler(client *mongo.Client, simulationsColl *mongo.Collection) gin.HandlerFunc {
	return func(c *gin.Context) {
		if coll, ok := validateSimulationAndGetDB(c, client, simulationsColl, "tracer_events"); ok {
			handler := GetBlockEndToEndLatencyHandler(coll)
			handler(c)
		}
	}
}

// GetSimulationVoteStatisticsHandler returns aggregated vote statistics by sender/receiver/type for a specific simulation
func GetSimulationVoteStatisticsHandler(client *mongo.Client, simulationsColl *mongo.Collection) gin.HandlerFunc {
	return func(c *gin.Context) {
		if coll, ok := validateSimulationAndGetDB(c, client, simulationsColl, "vote_latencies"); ok {
			handler := GetVoteStatisticsHandler(coll)
			handler(c)
		}
	}
}

// GetSimulationNetworkLatencyStatsHandler returns network latency statistics for a specific simulation
func GetSimulationNetworkLatencyStatsHandler(client *mongo.Client, simulationsColl *mongo.Collection) gin.HandlerFunc {
	return func(c *gin.Context) {
		if coll, ok := validateSimulationAndGetDB(c, client, simulationsColl, "network_latency_nodepair_summary"); ok {
			handler := GetNetworkLatencyStatsHandler(coll)
			handler(c)
		}
	}
}

// GetSimulationNetworkLatencyNodeStatsHandler returns network latency node statistics for a specific simulation
func GetSimulationNetworkLatencyNodeStatsHandler(client *mongo.Client, simulationsColl *mongo.Collection) gin.HandlerFunc {
	return func(c *gin.Context) {
		if coll, ok := validateSimulationAndGetDB(c, client, simulationsColl, "network_latency_node_stats"); ok {
			handler := GetNetworkLatencyNodeStatsHandler(coll)
			handler(c)
		}
	}
}

// GetSimulationNetworkLatencyOverviewHandler returns comprehensive network latency statistics for a specific simulation
func GetSimulationNetworkLatencyOverviewHandler(client *mongo.Client, simulationsColl *mongo.Collection) gin.HandlerFunc {
	return func(c *gin.Context) {
		if coll, ok := validateSimulationAndGetDB(c, client, simulationsColl, "network_latency_nodepair_summary"); ok {
			handler := GetNetworkLatencyOverviewHandler(coll)
			handler(c)
		}
	}
}

// GetSimulationConsensusEventsHandler returns consensus events for a specific simulation
func GetSimulationConsensusEventsHandler(client *mongo.Client, simulationsColl *mongo.Collection) gin.HandlerFunc {
	return func(c *gin.Context) {
		if coll, ok := validateSimulationAndGetDB(c, client, simulationsColl, "tracer_events"); ok {
			handler := GetConsensusEventsHandler(coll)
			handler(c)
		}
	}
}
