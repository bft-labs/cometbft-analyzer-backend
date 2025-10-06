package handlers

import (
	"context"
	"github.com/bft-labs/cometbft-analyzer-backend/types"
	"github.com/bft-labs/cometbft-analyzer-backend/utils"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"net/http"
	"strconv"
	"time"
)

func GetConsensusEventsHandler(collection *mongo.Collection) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract time window - only apply if explicitly provided
		fromStr := c.Query("from")
		toStr := c.Query("to")
		hasTimeFilter := fromStr != "" || toStr != ""

		var fromTime, toTime time.Time
		if hasTimeFilter {
			var err error
			fromTime, toTime, err = utils.TimeWindowFromContext(c)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid time range"})
				return
			}
		}

		// Parse pagination parameters - support both cursor and segment-based
		limit := 10000 // Default to 10000
		if limitStr := c.Query("limit"); limitStr != "" {
			if val, err := strconv.Atoi(limitStr); err == nil && val > 0 && val <= 50000 {
				limit = val
			}
		}

		cursor := c.Query("cursor")      // For forward pagination
		before := c.Query("before")      // For backward pagination
		segmentStr := c.Query("segment") // For segment-based pagination (1-indexed)
		includeTotalCount := c.Query("includeTotalCount") == "true"

		// Convert segment to skip/offset
		var skip int64 = 0
		if segmentStr != "" {
			if segment, err := strconv.Atoi(segmentStr); err == nil && segment > 0 {
				skip = int64((segment - 1) * limit) // segment is 1-indexed
			}
		}

		// Excluded event types (p2p events we don't want to show)
		excludedTypes := []string{
			"p2pProposal",
			"p2pProposalPOL",
			"p2pNewRoundStep",
			"p2pHasVote",
			"p2pVoteSetMaj23",
			"p2pVoteSetBits",
			"p2pHasProposalBlockPart",
		}

		matchConditions := bson.M{
			"type": bson.M{"$nin": excludedTypes},
		}

		// Add cursor-based pagination conditions
		timestampFilter := bson.M{}

		// Add time window filter if provided
		if hasTimeFilter {
			timestampFilter["$gte"] = fromTime
			timestampFilter["$lte"] = toTime
		}

		// Parse and add cursor conditions
		if cursor != "" {
			cursorTime, err := time.Parse(time.RFC3339, cursor)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cursor format, use RFC3339"})
				return
			}
			timestampFilter["$gt"] = cursorTime
		}

		if before != "" {
			beforeTime, err := time.Parse(time.RFC3339, before)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid before format, use RFC3339"})
				return
			}
			timestampFilter["$lt"] = beforeTime
		}

		if len(timestampFilter) > 0 {
			matchConditions["timestamp"] = timestampFilter
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Get total count only if requested (expensive operation)
		var totalCount *int
		if includeTotalCount {
			count, err := collection.CountDocuments(ctx, matchConditions)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count events"})
				return
			}
			countInt := int(count)
			totalCount = &countInt
		}

		matchStage := bson.D{{Key: "$match", Value: matchConditions}}

		// Fetch limit+1 to determine hasNext
		fetchLimit := limit + 1

		// Build pipeline based on pagination type
		pipeline := mongo.Pipeline{
			matchStage,
			bson.D{{Key: "$sort", Value: bson.D{{Key: "timestamp", Value: 1}}}},
		}

		// Add skip stage for segment-based pagination
		if skip > 0 {
			pipeline = append(pipeline, bson.D{{Key: "$skip", Value: skip}})
		}

		// Add limit stage
		pipeline = append(pipeline, bson.D{{Key: "$limit", Value: fetchLimit}})

		resultCursor, err := collection.Aggregate(ctx, pipeline)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch events"})
			return
		}
		defer resultCursor.Close(ctx)

		type eventWithTimestamp struct {
			event     types.EventResponse
			timestamp time.Time
		}

		var allEventsWithTimestamps []eventWithTimestamp

		for resultCursor.Next(ctx) {
			// Decode each document using type-aware decoder
			decodedEvent, err := types.DecodeConsensusEvent(resultCursor.Current)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to decode event: " + err.Error()})
				return
			}

			// Extract timestamp for cursor generation
			var doc bson.M
			var timestamp time.Time
			if err := bson.Unmarshal(resultCursor.Current, &doc); err == nil {
				if ts, ok := doc["timestamp"].(time.Time); ok {
					timestamp = ts
				}
			}

			allEventsWithTimestamps = append(allEventsWithTimestamps, eventWithTimestamp{
				event:     types.EventResponse{Event: decodedEvent},
				timestamp: timestamp,
			})
		}

		if err := resultCursor.Err(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "cursor error: " + err.Error()})
			return
		}

		// Determine hasNext and trim results if needed
		hasNext := len(allEventsWithTimestamps) > limit
		eventsToReturn := allEventsWithTimestamps
		if hasNext && len(allEventsWithTimestamps) > limit {
			eventsToReturn = allEventsWithTimestamps[:limit]
		}

		// Extract events and timestamps for response
		events := make([]types.EventResponse, len(eventsToReturn))
		for i, ewt := range eventsToReturn {
			events[i] = ewt.event
		}

		// Determine hasPrevious - true if we used cursor (meaning we're not at the beginning)
		hasPrevious := cursor != ""

		// Generate cursors
		var nextCursor, previousCursor *string
		if hasNext && len(eventsToReturn) > 0 {
			lastTimestamp := eventsToReturn[len(eventsToReturn)-1].timestamp
			if !lastTimestamp.IsZero() {
				nextStr := lastTimestamp.Format(time.RFC3339)
				nextCursor = &nextStr
			}
		}
		if hasPrevious && len(eventsToReturn) > 0 {
			firstTimestamp := eventsToReturn[0].timestamp
			if !firstTimestamp.IsZero() {
				prevStr := firstTimestamp.Format(time.RFC3339)
				previousCursor = &prevStr
			}
		}

		response := types.PaginatedEventsResponse{
			Data: events,
			Pagination: types.CursorPaginationMeta{
				Limit:          limit,
				HasNext:        hasNext,
				HasPrevious:    hasPrevious,
				NextCursor:     nextCursor,
				PreviousCursor: previousCursor,
				TotalCount:     totalCount,
			},
		}

		c.JSON(http.StatusOK, response)
	}
}
