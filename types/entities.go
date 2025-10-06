package types

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

// User represents a user in the system
type User struct {
	ID        primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Username  string             `json:"username" bson:"username"`
	Email     string             `json:"email" bson:"email"`
	CreatedAt time.Time          `json:"createdAt" bson:"createdAt"`
	UpdatedAt time.Time          `json:"updatedAt" bson:"updatedAt"`
}

// Project represents a project owned by a user
type Project struct {
	ID          primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Name        string             `json:"name" bson:"name"`
	Description string             `json:"description" bson:"description"`
	UserID      primitive.ObjectID `json:"userId" bson:"userId"`
	CreatedAt   time.Time          `json:"createdAt" bson:"createdAt"`
	UpdatedAt   time.Time          `json:"updatedAt" bson:"updatedAt"`
}

// SimulationStatus represents the overall status of a simulation
type SimulationStatus string

const (
	SimulationStatusLogFileRequired SimulationStatus = "logfile_required"
	SimulationStatusProcessing      SimulationStatus = "processing"
	SimulationStatusProcessed       SimulationStatus = "processed"
	SimulationStatusFailed          SimulationStatus = "failed"
)

// ProcessingStatus represents the status of simulation processing
type ProcessingStatus string

const (
	ProcessingStatusPending    ProcessingStatus = "pending"
	ProcessingStatusProcessing ProcessingStatus = "processing"
	ProcessingStatusCompleted  ProcessingStatus = "completed"
	ProcessingStatusFailed     ProcessingStatus = "failed"
)

// LogFileInfo represents metadata for an uploaded log file
type LogFileInfo struct {
	OriginalFilename string    `json:"originalFilename" bson:"originalFilename"`
	FilePath         string    `json:"filePath" bson:"filePath"`
	FileSize         int64     `json:"fileSize" bson:"fileSize"`
	UploadedAt       time.Time `json:"uploadedAt" bson:"uploadedAt"`
}

// ProcessingResult represents the result of processing log files
type ProcessingResult struct {
	ProcessedFiles int       `json:"processedFiles" bson:"processedFiles"`
	TotalFiles     int       `json:"totalFiles" bson:"totalFiles"`
	ProcessingTime int64     `json:"processingTime" bson:"processingTime"` // in milliseconds
	ErrorMessage   string    `json:"errorMessage,omitempty" bson:"errorMessage,omitempty"`
	ProcessedAt    time.Time `json:"processedAt" bson:"processedAt"`
}

// Simulation represents a simulation within a project
type Simulation struct {
	ID               primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Name             string             `json:"name" bson:"name"`
	Description      string             `json:"description" bson:"description"`
	ProjectID        primitive.ObjectID `json:"projectId" bson:"projectId"`
	UserID           primitive.ObjectID `json:"userId" bson:"userId"`
	LogFiles         []LogFileInfo      `json:"logFiles,omitempty" bson:"logFiles,omitempty"`
	Status           SimulationStatus   `json:"status" bson:"status"`
	ProcessingStatus ProcessingStatus   `json:"processingStatus,omitempty" bson:"processingStatus,omitempty"`
	ProcessingResult *ProcessingResult  `json:"processingResult,omitempty" bson:"processingResult,omitempty"`
	CreatedAt        time.Time          `json:"createdAt" bson:"createdAt"`
	UpdatedAt        time.Time          `json:"updatedAt" bson:"updatedAt"`
}

// CreateUserRequest represents the request body for creating a user
type CreateUserRequest struct {
	Username string `json:"username" binding:"required,min=3,max=30,alphanum"`
	Email    string `json:"email" binding:"required,email"`
}

// CreateProjectRequest represents the request body for creating a project
type CreateProjectRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

// UpdateProjectRequest represents the request body for updating a project
type UpdateProjectRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

// CreateSimulationRequest represents the request body for creating a simulation
type CreateSimulationRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

// UpdateSimulationRequest represents the request body for updating a simulation
type UpdateSimulationRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

// SimulationResponse represents the response structure for simulation endpoints
type SimulationResponse struct {
	ID               primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Name             string             `json:"name" bson:"name"`
	Description      string             `json:"description" bson:"description"`
	ProjectID        primitive.ObjectID `json:"projectId" bson:"projectId"`
	UserID           primitive.ObjectID `json:"userId" bson:"userId"`
	LogFiles         []LogFileInfo      `json:"logFiles,omitempty" bson:"logFiles,omitempty"`
	Status           SimulationStatus   `json:"status" bson:"status"`
	ProcessingStatus ProcessingStatus   `json:"processingStatus,omitempty" bson:"processingStatus,omitempty"`
	ProcessingResult *ProcessingResult  `json:"processingResult,omitempty" bson:"processingResult,omitempty"`
	CreatedAt        time.Time          `json:"createdAt" bson:"createdAt"`
	UpdatedAt        time.Time          `json:"updatedAt" bson:"updatedAt"`
}

// GetLogFilePaths returns just the file paths for backward compatibility
func (s *Simulation) GetLogFilePaths() []string {
	paths := make([]string, len(s.LogFiles))
	for i, logFile := range s.LogFiles {
		paths[i] = logFile.FilePath
	}
	return paths
}

// HasLogFiles returns true if the simulation has any log files
func (s *Simulation) HasLogFiles() bool {
	return len(s.LogFiles) > 0
}

// LogFileCount returns the number of log files
func (s *Simulation) LogFileCount() int {
	return len(s.LogFiles)
}

// ToResponse converts a Simulation to SimulationResponse (excludes database field)
func (s *Simulation) ToResponse() SimulationResponse {
	return SimulationResponse{
		ID:               s.ID,
		Name:             s.Name,
		Description:      s.Description,
		ProjectID:        s.ProjectID,
		UserID:           s.UserID,
		LogFiles:         s.LogFiles,
		Status:           s.Status,
		ProcessingStatus: s.ProcessingStatus,
		ProcessingResult: s.ProcessingResult,
		CreatedAt:        s.CreatedAt,
		UpdatedAt:        s.UpdatedAt,
	}
}
