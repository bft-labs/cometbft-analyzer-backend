package types

import (
	"encoding/json"
	"fmt"
	"github.com/bft-labs/cometbft-analyzer-types/pkg/events"
	"go.mongodb.org/mongo-driver/bson"
)

// EventResponse wraps any consensus event for API responses
type EventResponse struct {
	Event any `json:"event"`
}

// MarshalJSON implements custom JSON marshaling to flatten the event structure
func (er EventResponse) MarshalJSON() ([]byte, error) {
	return json.Marshal(er.Event)
}

// DecodeConsensusEvent decodes a MongoDB document into the appropriate event type
func DecodeConsensusEvent(raw bson.Raw) (any, error) {
	// First decode to get the type field
	var baseDoc struct {
		EventType string `bson:"eventType"`
	}

	if err := bson.Unmarshal(raw, &baseDoc); err != nil {
		return nil, fmt.Errorf("failed to decode base document: %w", err)
	}

	// Switch on event type and decode into specific struct
	switch baseDoc.EventType {
	// P2P message events (confirmed exchanges)
	case "p2pVote":
		var event events.EventP2pVote
		if err := bson.Unmarshal(raw, &event); err != nil {
			return nil, fmt.Errorf("failed to decode EventP2pVote: %w", err)
		}
		return &event, nil

	case "p2pBlockPart":
		var event events.EventP2pBlockPart
		if err := bson.Unmarshal(raw, &event); err != nil {
			return nil, fmt.Errorf("failed to decode EventP2pBlockPart: %w", err)
		}
		return &event, nil

	case "p2pProposal":
		var event events.EventP2pProposal
		if err := bson.Unmarshal(raw, &event); err != nil {
			return nil, fmt.Errorf("failed to decode EventP2pProposal: %w", err)
		}
		return &event, nil

	// TODO: Uncomment and use these when needed from frontend
	//case "p2pProposalPOL":
	//	var event events.EventP2pProposalPOL
	//	if err := bson.Unmarshal(raw, &event); err != nil {
	//		return nil, fmt.Errorf("failed to decode EventP2pProposalPOL: %w", err)
	//	}
	//	return &event, nil
	//
	//case "p2pNewRoundStep":
	//	var event events.EventP2pNewRoundStep
	//	if err := bson.Unmarshal(raw, &event); err != nil {
	//		return nil, fmt.Errorf("failed to decode EventP2pNewRoundStep: %w", err)
	//	}
	//	return &event, nil
	//
	//case "p2pHasVote":
	//	var event events.EventP2pHasVote
	//	if err := bson.Unmarshal(raw, &event); err != nil {
	//		return nil, fmt.Errorf("failed to decode EventP2pHasVote: %w", err)
	//	}
	//	return &event, nil
	//
	//case "p2pVoteSetMaj23":
	//	var event events.EventP2pVoteSetMaj23
	//	if err := bson.Unmarshal(raw, &event); err != nil {
	//		return nil, fmt.Errorf("failed to decode EventP2pVoteSetMaj23: %w", err)
	//	}
	//	return &event, nil
	//
	//case "p2pVoteSetBits":
	//	var event events.EventP2pVoteSetBits
	//	if err := bson.Unmarshal(raw, &event); err != nil {
	//		return nil, fmt.Errorf("failed to decode EventP2pVoteSetBits: %w", err)
	//	}
	//	return &event, nil
	//
	//case "p2pHasProposalBlockPart":
	//	var event events.EventP2pBlockPart // Reuse EventP2pBlockPart for this
	//	if err := bson.Unmarshal(raw, &event); err != nil {
	//		return nil, fmt.Errorf("failed to decode p2pHasProposalBlockPart: %w", err)
	//	}
	//	return &event, nil

	// Consensus step events
	case "enteringNewRound":
		var event events.EventEnteringNewRound
		if err := bson.Unmarshal(raw, &event); err != nil {
			return nil, fmt.Errorf("failed to decode EventEnteringNewRound: %w", err)
		}
		return &event, nil

	case "proposeStep":
		var event events.EventProposeStep
		if err := bson.Unmarshal(raw, &event); err != nil {
			return nil, fmt.Errorf("failed to decode EventProposeStep: %w", err)
		}
		return &event, nil

	case "enteringPrevoteStep":
		var event events.EventEnteringPrevoteStep
		if err := bson.Unmarshal(raw, &event); err != nil {
			return nil, fmt.Errorf("failed to decode EventEnteringPrevoteStep: %w", err)
		}
		return &event, nil

	case "enteringPrecommitStep":
		var event events.EventEnteringPrecommitStep
		if err := bson.Unmarshal(raw, &event); err != nil {
			return nil, fmt.Errorf("failed to decode EventEnteringPrecommitStep: %w", err)
		}
		return &event, nil

	case "enteringPrevoteWaitStep":
		var event events.EventEnteringPrevoteWaitStep
		if err := bson.Unmarshal(raw, &event); err != nil {
			return nil, fmt.Errorf("failed to decode EventEnteringPrevoteWaitStep: %w", err)
		}
		return &event, nil

	case "enteringPrecommitWaitStep":
		var event events.EventEnteringPrecommitWaitStep
		if err := bson.Unmarshal(raw, &event); err != nil {
			return nil, fmt.Errorf("failed to decode EventEnteringPrecommitWaitStep: %w", err)
		}
		return &event, nil

	case "enteringCommitStep":
		var event events.EventEnteringCommitStep
		if err := bson.Unmarshal(raw, &event); err != nil {
			return nil, fmt.Errorf("failed to decode EventEnteringCommitStep: %w", err)
		}
		return &event, nil

	case "enteringWaitStep":
		var event events.EventEnteringWaitStep
		if err := bson.Unmarshal(raw, &event); err != nil {
			return nil, fmt.Errorf("failed to decode EventEnteringWaitStep: %w", err)
		}
		return &event, nil

	// Other consensus events
	case "receivedProposal":
		var event events.EventReceivedProposal
		if err := bson.Unmarshal(raw, &event); err != nil {
			return nil, fmt.Errorf("failed to decode EventReceivedProposal: %w", err)
		}
		return &event, nil

	case "receivedCompleteProposalBlock":
		var event events.EventReceivedCompleteProposalBlock
		if err := bson.Unmarshal(raw, &event); err != nil {
			return nil, fmt.Errorf("failed to decode EventReceivedCompleteProposalBlock: %w", err)
		}
		return &event, nil

	case "scheduledTimeout":
		var event events.EventScheduledTimeout
		if err := bson.Unmarshal(raw, &event); err != nil {
			return nil, fmt.Errorf("failed to decode EventScheduledTimeout: %w", err)
		}
		return &event, nil

	default:
		// For unrecognized types, decode into a generic BaseEvent
		var event events.BaseEvent
		if err := bson.Unmarshal(raw, &event); err != nil {
			return nil, fmt.Errorf("failed to decode BaseEvent for type %s: %w", baseDoc.EventType, err)
		}
		return &event, nil
	}
}

// NetworkLatencyOverviewResponse represents comprehensive network latency statistics
type NetworkLatencyOverviewResponse struct {
	OverallStats                   OverallLatencyStats    `json:"overallStats"`
	OverallWeightedAvgP95LatencyMs float64                `json:"overallWeightedAvgP95LatencyMs"`
	MessageTypeWithHighestAvgP95   MessageTypeLatencyInfo `json:"messageTypeWithHighestAvgP95"`
	NodeWithHighestAvgP95          NodeLatencyInfo        `json:"nodeWithHighestAvgP95"`
	MessageTypeLatency             map[string]float64     `json:"messageTypeLatency"`
	NodeLatencyContribution        map[string]float64     `json:"nodeLatencyContribution"`
}

type OverallLatencyStats struct {
	Count            int     `json:"count"`
	P50ToP95Count    int     `json:"p50ToP95Count"`
	WeightedAvgP95Ms float64 `json:"weightedAvgP95Ms"`
}

type MessageTypeLatencyInfo struct {
	MessageType string  `json:"messageType"`
	LatencyMs   float64 `json:"latencyMs"`
}

type NodeLatencyInfo struct {
	NodeId    string  `json:"nodeId"`
	LatencyMs float64 `json:"latencyMs"`
}

// PaginatedEventsResponse wraps events with cursor-based pagination metadata
type PaginatedEventsResponse struct {
	Data       []EventResponse      `json:"data"`
	Pagination CursorPaginationMeta `json:"pagination"`
}

// CursorPaginationMeta contains cursor-based pagination metadata
type CursorPaginationMeta struct {
	Limit          int     `json:"limit"`
	HasNext        bool    `json:"hasNext"`
	HasPrevious    bool    `json:"hasPrevious"`
	NextCursor     *string `json:"nextCursor"`
	PreviousCursor *string `json:"previousCursor"`
	TotalCount     *int    `json:"totalCount"` // Optional, expensive to calculate
}
