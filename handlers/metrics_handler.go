package handlers

import (
	"context"
	"github.com/bft-labs/cometbft-analyzer-backend/metrics"
	"github.com/bft-labs/cometbft-analyzer-backend/types"
	"github.com/bft-labs/cometbft-analyzer-backend/utils"
	"github.com/bft-labs/cometbft-analyzer-types/pkg/statistics/latency"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"net/http"
	"strconv"
	"time"
)

// GetVoteLatenciesHandler returns paginated vote latencies for the given time range
func GetVoteLatenciesHandler(coll *mongo.Collection) gin.HandlerFunc {
	return func(c *gin.Context) {
		from, to, err := utils.TimeWindowFromContext(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid time range"})
			return
		}

		// Parse pagination parameters
		page := 1
		if pageStr := c.Query("page"); pageStr != "" {
			if parsedPage, err := strconv.Atoi(pageStr); err == nil && parsedPage > 0 {
				page = parsedPage
			}
		}

		perPage := 100 // Default per page
		if perPageStr := c.Query("perPage"); perPageStr != "" {
			if parsedPerPage, err := strconv.Atoi(perPageStr); err == nil && parsedPerPage > 0 && parsedPerPage <= 1000 {
				perPage = parsedPerPage
			}
		}

		// Parse percentile threshold parameter
		threshold := "p95" // Default to p95
		if thresholdStr := c.Query("threshold"); thresholdStr != "" {
			switch thresholdStr {
			case "p50", "p95", "p99":
				threshold = thresholdStr
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		result, err := metrics.GetVoteLatencies(ctx, coll, from, to, page, perPage, threshold)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		data := make([]types.VoteLatencyResponse, len(result.Data))
		for i, v := range result.Data {
			data[i] = types.VoteLatencyResponse{
				Height:       v.Vote.Height,
				Round:        v.Vote.Round,
				Type:         v.Vote.Type,
				ValidatorIdx: v.Vote.ValidatorIndex,
				Sender:       v.SenderPeerId,
				Receiver:     v.RecipientPeerId,
				SentTime:     v.SentTime,
				ReceivedTime: v.ReceivedTime,
				LatencyMs:    float64(v.Latency) / float64(time.Millisecond),
			}
		}

		// Calculate total pages
		totalPages := (result.Total + perPage - 1) / perPage

		response := types.PaginatedVoteLatencyResponse{
			Data: data,
			Pagination: types.PaginationMeta{
				Page:       page,
				PerPage:    perPage,
				Total:      result.Total,
				TotalPages: totalPages,
			},
		}

		c.JSON(http.StatusOK, response)
	}
}

// GetPairLatencyHandler returns sender→receiver latency percentiles
func GetPairLatencyHandler(coll *mongo.Collection) gin.HandlerFunc {
	return func(c *gin.Context) {
		from, to, err := utils.TimeWindowFromContext(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid time range"})
			return
		}

		// TODO: pass window into vizmetrics if supported
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		data, err := metrics.ComputePairwiseLatencyPercentiles(ctx, coll, from, to)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, data)
	}
}

// GetBlockLatencyTimeSeriesHandler returns per-block latency time-series
func GetBlockLatencyTimeSeriesHandler(coll *mongo.Collection) gin.HandlerFunc {
	return func(c *gin.Context) {
		from, to, err := utils.TimeWindowFromContext(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid time range"})
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		data, err := metrics.ComputeBlockLatencyTimeSeries(ctx, coll, from, to)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, data)
	}
}

// GetLatencyStatsHandler returns histogram and jitter stats
func GetLatencyStatsHandler(coll *mongo.Collection) gin.HandlerFunc {
	return func(c *gin.Context) {
		from, to, err := utils.TimeWindowFromContext(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid time range"})
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		stats, err := metrics.ComputeLatencyStats(ctx, coll, from, to)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, stats)
	}
}

// GetMessageSuccessRateHandler returns send vs receive counts and delivery ratio
func GetMessageSuccessRateHandler(coll *mongo.Collection) gin.HandlerFunc {
	return func(c *gin.Context) {
		from, to, err := utils.TimeWindowFromContext(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid time range"})
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		rates, err := metrics.ComputeMessageSuccessRate(ctx, coll, from, to)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, rates)
	}
}

// GetBlockEndToEndLatencyHandler returns end-to-end consensus latency per block height
func GetBlockEndToEndLatencyHandler(coll *mongo.Collection) gin.HandlerFunc {
	return func(c *gin.Context) {
		from, to, err := utils.TimeWindowFromContext(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid time range"})
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		stats, err := metrics.ComputeBlockEndToEndLatencyByHeight(ctx, coll, from, to)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, stats)
	}
}

// GetVoteStatisticsHandler returns aggregated vote statistics by sender/receiver/type
func GetVoteStatisticsHandler(coll *mongo.Collection) gin.HandlerFunc {
	return func(c *gin.Context) {
		from, to, err := utils.TimeWindowFromContext(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid time range"})
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		stats, err := metrics.ComputeVoteStatistics(ctx, coll, from, to)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, stats)
	}
}

// GetNetworkLatencyStatsHandler returns network latency statistics
func GetNetworkLatencyStatsHandler(coll *mongo.Collection) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		stats, err := metrics.GetNetworkLatencyStats(ctx, coll)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, stats)
	}
}

// GetNetworkLatencyNodeStatsHandler returns network latency node statistics
func GetNetworkLatencyNodeStatsHandler(coll *mongo.Collection) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		cursor, err := coll.Find(ctx, bson.M{})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer cursor.Close(ctx)

		var nodeStats []latency.NodeNetworkStats
		if err = cursor.All(ctx, &nodeStats); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, nodeStats)
	}
}

// GetNetworkLatencyOverviewHandler returns comprehensive network latency statistics
func GetNetworkLatencyOverviewHandler(coll *mongo.Collection) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		stats, err := metrics.GetNetworkLatencyOverview(ctx, coll)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, stats)
	}
}
