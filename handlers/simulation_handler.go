package handlers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/bft-labs/cometbft-analyzer-backend/types"
	"github.com/bft-labs/cometbft-analyzer-backend/utils"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// CreateSimulationHandler creates a new simulation
func CreateSimulationHandler(collection *mongo.Collection) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("projectId")
		projectObjectID, err := primitive.ObjectIDFromHex(projectID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
			return
		}

		userID := c.Param("userId")
		userObjectID, err := primitive.ObjectIDFromHex(userID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
			return
		}

		// Check if this is multipart form data (with potential file upload)
		contentType := c.GetHeader("Content-Type")
		var req types.CreateSimulationRequest
		var logFiles []types.LogFileInfo

		if contentType != "" && contentType[:19] == "multipart/form-data" {
			// Handle multipart form data
			req.Name = c.PostForm("name")
			req.Description = c.PostForm("description")

			if req.Name == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
				return
			}

			// Handle multiple log file uploads
			form, err := c.MultipartForm()
			if err == nil && form.File["logfiles"] != nil {
				files := form.File["logfiles"]
				for i, fileHeader := range files {
					// Open the file
					file, err := fileHeader.Open()
					if err != nil {
						// Clean up previously uploaded files
						for _, logFile := range logFiles {
							os.Remove(logFile.FilePath)
						}
						c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read uploaded file"})
						return
					}
					defer file.Close()

					// Generate temporary filename (will be updated after simulation creation)
					tempFilename := fmt.Sprintf("temp_%d_%d_%s", time.Now().UnixNano(), i, fileHeader.Filename)
					filePath := filepath.Join("uploads", tempFilename)

					// Ensure temp directory exists
					if err := os.MkdirAll("uploads", 0755); err != nil {
						c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create uploads directory"})
						return
					}

					// Create destination file
					dst, err := os.Create(filePath)
					if err != nil {
						// Clean up previously uploaded files
						for _, logFile := range logFiles {
							os.Remove(logFile.FilePath)
						}
						c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create file"})
						return
					}

					// Copy file content
					if _, err := io.Copy(dst, file); err != nil {
						dst.Close()
						// Clean up all uploaded files including current one
						os.Remove(filePath)
						for _, logFile := range logFiles {
							os.Remove(logFile.FilePath)
						}
						c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
						return
					}
					dst.Close()

					// Create LogFileInfo with metadata
					logFileInfo := types.LogFileInfo{
						OriginalFilename: fileHeader.Filename,
						FilePath:         filePath,
						FileSize:         fileHeader.Size,
						UploadedAt:       time.Now(),
					}
					logFiles = append(logFiles, logFileInfo)
				}
			}
		} else {
			// Handle JSON request
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}

		// Determine initial status based on log files
		var initialStatus types.SimulationStatus
		var initialProcessingStatus types.ProcessingStatus

		if len(logFiles) > 0 {
			initialStatus = types.SimulationStatusProcessing
			initialProcessingStatus = types.ProcessingStatusPending
		} else {
			initialStatus = types.SimulationStatusLogFileRequired
		}

		simulation := types.Simulation{
			Name:             req.Name,
			Description:      req.Description,
			ProjectID:        projectObjectID,
			UserID:           userObjectID,
			LogFiles:         logFiles,
			Status:           initialStatus,
			ProcessingStatus: initialProcessingStatus,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}

		result, err := collection.InsertOne(context.Background(), simulation)
		if err != nil {
			// Clean up uploaded files if database insert fails
			for _, logFile := range logFiles {
				os.Remove(logFile.FilePath)
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}

		simulation.ID = result.InsertedID.(primitive.ObjectID)

		// Move uploaded files to simulation directory
		if len(logFiles) > 0 {
			simulationDir, err := utils.EnsureSimulationDir(userObjectID, projectObjectID, simulation.ID)
			if err != nil {
				// Clean up temp files and fail
				for _, logFile := range logFiles {
					os.Remove(logFile.FilePath)
				}
				collection.DeleteOne(context.Background(), bson.M{"_id": simulation.ID})
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create simulation directory"})
				return
			}

			var updatedLogFiles []types.LogFileInfo
			for i, logFile := range logFiles {
				filename := fmt.Sprintf("%d_%s", i, logFile.OriginalFilename)
				newFilePath := filepath.Join(simulationDir, filename)

				if err := os.Rename(logFile.FilePath, newFilePath); err == nil {
					// Update LogFileInfo with new path
					updatedLogFile := logFile
					updatedLogFile.FilePath = newFilePath
					updatedLogFiles = append(updatedLogFiles, updatedLogFile)
				} else {
					// Clean up on failure
					for _, updatedFile := range updatedLogFiles {
						os.Remove(updatedFile.FilePath)
					}
					os.Remove(logFile.FilePath)
					collection.DeleteOne(context.Background(), bson.M{"_id": simulation.ID})
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to move files to simulation directory"})
					return
				}
			}

			// Update simulation with new file info
			simulation.LogFiles = updatedLogFiles
			collection.UpdateOne(context.Background(), bson.M{"_id": simulation.ID}, bson.M{
				"$set": bson.M{"logFiles": updatedLogFiles},
			})

			// If files were uploaded during creation, start processing automatically
			if len(updatedLogFiles) > 0 && simulation.Status == types.SimulationStatusProcessing {
				go processSimulationLogs(collection, simulation)
			}
		}

		c.JSON(http.StatusCreated, simulation.ToResponse())
	}
}

// GetSimulationHandler retrieves a simulation by ID
func GetSimulationHandler(collection *mongo.Collection) gin.HandlerFunc {
	return func(c *gin.Context) {
		simulationID := c.Param("id")
		objectID, err := primitive.ObjectIDFromHex(simulationID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid simulation ID"})
			return
		}

		var simulation types.Simulation
		err = collection.FindOne(context.Background(), bson.M{"_id": objectID}).Decode(&simulation)
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "Simulation not found"})
			return
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}

		c.JSON(http.StatusOK, simulation.ToResponse())
	}
}

// GetSimulationsByProjectHandler retrieves all simulations for a specific project
func GetSimulationsByProjectHandler(collection *mongo.Collection) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("projectId")
		projectObjectID, err := primitive.ObjectIDFromHex(projectID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
			return
		}

		cursor, err := collection.Find(context.Background(), bson.M{"projectId": projectObjectID})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}
		defer cursor.Close(context.Background())

		var simulations []types.Simulation
		if err := cursor.All(context.Background(), &simulations); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode simulations"})
			return
		}

		// Convert to response format
		responses := make([]types.SimulationResponse, len(simulations))
		for i, sim := range simulations {
			responses[i] = sim.ToResponse()
		}

		c.JSON(http.StatusOK, responses)
	}
}

// GetSimulationsByUserHandler retrieves all simulations for a specific user
func GetSimulationsByUserHandler(collection *mongo.Collection) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.Param("userId")
		userObjectID, err := primitive.ObjectIDFromHex(userID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
			return
		}

		cursor, err := collection.Find(context.Background(), bson.M{"userId": userObjectID})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}
		defer cursor.Close(context.Background())

		var simulations []types.Simulation
		if err := cursor.All(context.Background(), &simulations); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode simulations"})
			return
		}

		// Convert to response format
		responses := make([]types.SimulationResponse, len(simulations))
		for i, sim := range simulations {
			responses[i] = sim.ToResponse()
		}

		c.JSON(http.StatusOK, responses)
	}
}

// UpdateSimulationHandler updates a simulation by ID
func UpdateSimulationHandler(collection *mongo.Collection) gin.HandlerFunc {
	return func(c *gin.Context) {
		simulationID := c.Param("id")
		objectID, err := primitive.ObjectIDFromHex(simulationID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid simulation ID"})
			return
		}

		var req types.UpdateSimulationRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		update := bson.M{
			"$set": bson.M{
				"updatedAt": time.Now(),
			},
		}

		if req.Name != nil {
			update["$set"].(bson.M)["name"] = *req.Name
		}
		if req.Description != nil {
			update["$set"].(bson.M)["description"] = *req.Description
		}

		result, err := collection.UpdateOne(context.Background(), bson.M{"_id": objectID}, update)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}

		if result.MatchedCount == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Simulation not found"})
			return
		}

		var simulation types.Simulation
		err = collection.FindOne(context.Background(), bson.M{"_id": objectID}).Decode(&simulation)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve updated simulation"})
			return
		}

		c.JSON(http.StatusOK, simulation.ToResponse())
	}
}

// DeleteSimulationHandler deletes a simulation by ID
func DeleteSimulationHandler(collection *mongo.Collection) gin.HandlerFunc {
	return func(c *gin.Context) {
		simulationID := c.Param("id")
		objectID, err := primitive.ObjectIDFromHex(simulationID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid simulation ID"})
			return
		}

		// Get simulation to check for log file
		var simulation types.Simulation
		err = collection.FindOne(context.Background(), bson.M{"_id": objectID}).Decode(&simulation)
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "Simulation not found"})
			return
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}

		// Delete log files if they exist
		for _, logFile := range simulation.LogFiles {
			if logFile.FilePath != "" {
				if err := os.Remove(logFile.FilePath); err != nil {
					// Log error but don't fail the deletion
					fmt.Printf("Failed to delete log file %s: %v\n", logFile.FilePath, err)
				}
			}
		}

		result, err := collection.DeleteOne(context.Background(), bson.M{"_id": objectID})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}

		if result.DeletedCount == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Simulation not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Simulation deleted successfully"})
	}
}

// UploadLogFileHandler uploads a log file for a simulation
func UploadLogFileHandler(collection *mongo.Collection) gin.HandlerFunc {
	return func(c *gin.Context) {
		simulationID := c.Param("id")
		objectID, err := primitive.ObjectIDFromHex(simulationID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid simulation ID"})
			return
		}

		// Check if simulation exists
		var simulation types.Simulation
		err = collection.FindOne(context.Background(), bson.M{"_id": objectID}).Decode(&simulation)
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "Simulation not found"})
			return
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}

		// Handle multiple log file uploads
		form, err := c.MultipartForm()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse multipart form"})
			return
		}

		files := form.File["logfiles"]
		if len(files) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No log files provided"})
			return
		}

		// Get simulation directory
		simulationDir, err := utils.EnsureSimulationDir(simulation.UserID, simulation.ProjectID, simulation.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create simulation directory"})
			return
		}

		var newLogFiles []types.LogFileInfo

		// Process each uploaded file
		for i, fileHeader := range files {
			// Open the file
			file, err := fileHeader.Open()
			if err != nil {
				// Clean up previously uploaded files
				for _, logFile := range newLogFiles {
					os.Remove(logFile.FilePath)
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read uploaded file"})
				return
			}
			defer file.Close()

			// Generate unique filename
			filename := fmt.Sprintf("%d_%s", len(simulation.LogFiles)+i, fileHeader.Filename)
			filePath := filepath.Join(simulationDir, filename)

			// Create destination file
			dst, err := os.Create(filePath)
			if err != nil {
				// Clean up previously uploaded files
				for _, logFile := range newLogFiles {
					os.Remove(logFile.FilePath)
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create file"})
				return
			}

			// Copy file content
			if _, err := io.Copy(dst, file); err != nil {
				dst.Close()
				// Clean up all uploaded files including current one
				os.Remove(filePath)
				for _, logFile := range newLogFiles {
					os.Remove(logFile.FilePath)
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
				return
			}
			dst.Close()

			// Create LogFileInfo with metadata
			logFileInfo := types.LogFileInfo{
				OriginalFilename: fileHeader.Filename,
				FilePath:         filePath,
				FileSize:         fileHeader.Size,
				UploadedAt:       time.Now(),
			}
			newLogFiles = append(newLogFiles, logFileInfo)
		}

		// Add new files to existing ones
		allLogFiles := append(simulation.LogFiles, newLogFiles...)

		// Update status if this is the first upload
		var newStatus types.SimulationStatus = simulation.Status
		var newProcessingStatus types.ProcessingStatus = simulation.ProcessingStatus

		if simulation.Status == types.SimulationStatusLogFileRequired && len(allLogFiles) > 0 {
			newStatus = types.SimulationStatusProcessing
			newProcessingStatus = types.ProcessingStatusPending
		}

		// Update simulation with new files and status
		update := bson.M{
			"$set": bson.M{
				"logFiles":         allLogFiles,
				"status":           newStatus,
				"processingStatus": newProcessingStatus,
				"updatedAt":        time.Now(),
			},
		}

		_, err = collection.UpdateOne(context.Background(), bson.M{"_id": objectID}, update)
		if err != nil {
			// Clean up uploaded files if database update fails
			for _, logFile := range newLogFiles {
				os.Remove(logFile.FilePath)
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}

		// Create response with original filenames
		uploadedFileNames := make([]string, len(newLogFiles))
		for i, logFile := range newLogFiles {
			uploadedFileNames[i] = logFile.OriginalFilename
		}

		c.JSON(http.StatusOK, gin.H{
			"message":           "Log files uploaded successfully",
			"uploadedFiles":     len(newLogFiles),
			"totalFiles":        len(allLogFiles),
			"uploadedFileNames": uploadedFileNames,
		})
	}
}

// ProcessSimulationHandler processes log files for a simulation
func ProcessSimulationHandler(collection *mongo.Collection) gin.HandlerFunc {
	return func(c *gin.Context) {
		simulationID := c.Param("id")
		objectID, err := primitive.ObjectIDFromHex(simulationID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid simulation ID"})
			return
		}

		// Check if simulation exists
		var simulation types.Simulation
		err = collection.FindOne(context.Background(), bson.M{"_id": objectID}).Decode(&simulation)
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "Simulation not found"})
			return
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}

		// Check if log files exist
		if !simulation.HasLogFiles() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No log files available for processing"})
			return
		}

		// Check if already processing
		if simulation.ProcessingStatus == types.ProcessingStatusProcessing {
			c.JSON(http.StatusConflict, gin.H{"error": "Simulation is already being processed"})
			return
		}

		// Update status to processing
		update := bson.M{
			"$set": bson.M{
				"processingStatus": types.ProcessingStatusProcessing,
				"updatedAt":        time.Now(),
			},
		}
		_, err = collection.UpdateOne(context.Background(), bson.M{"_id": objectID}, update)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update processing status"})
			return
		}

		// Process asynchronously
		go func() {
			startTime := time.Now()
			processSimulationAsync(collection, simulation)
			processingTime := time.Since(startTime).Milliseconds()

			// Get simulation directory for cometbft-log-etl
			simulationDir := utils.GetSimulationDir(simulation.UserID, simulation.ProjectID, simulation.ID)

			// Execute cometbft-log-etl with simulation ID
			cmd := exec.Command("cometbft-log-etl", "-dir", simulationDir, "-simulation", simulation.ID.Hex())
			err := cmd.Run()

			var processingResult types.ProcessingResult
			var status types.ProcessingStatus

			var simulationStatus types.SimulationStatus

			if err != nil {
				// Processing failed
				status = types.ProcessingStatusFailed
				simulationStatus = types.SimulationStatusFailed
				processingResult = types.ProcessingResult{
					ProcessedFiles: 0,
					TotalFiles:     simulation.LogFileCount(),
					ProcessingTime: processingTime,
					ErrorMessage:   fmt.Sprintf("Parser execution failed: %v"),
					ProcessedAt:    time.Now(),
				}
			} else {
				// Processing succeeded
				status = types.ProcessingStatusCompleted
				simulationStatus = types.SimulationStatusProcessed
				processingResult = types.ProcessingResult{
					ProcessedFiles: simulation.LogFileCount(),
					TotalFiles:     simulation.LogFileCount(),
					ProcessingTime: processingTime,
					ProcessedAt:    time.Now(),
				}

				// Create processed directory for future output files
				_, dirErr := utils.EnsureProcessedDir(simulation.UserID, simulation.ProjectID, simulation.ID)
				if dirErr != nil {
					fmt.Printf("Warning: Failed to create processed directory: %v\n", dirErr)
				}
			}

			// Update simulation with final result
			finalUpdate := bson.M{
				"$set": bson.M{
					"status":           simulationStatus,
					"processingStatus": status,
					"processingResult": processingResult,
					"updatedAt":        time.Now(),
				},
			}
			collection.UpdateOne(context.Background(), bson.M{"_id": objectID}, finalUpdate)
		}()

		c.JSON(http.StatusAccepted, gin.H{
			"message":      "Simulation processing started",
			"simulationId": simulationID,
			"status":       "processing",
		})
	}
}

// processSimulationLogs processes log files for a simulation
func processSimulationLogs(collection *mongo.Collection, simulation types.Simulation) {
	startTime := time.Now()

	// Update status to processing
	update := bson.M{
		"$set": bson.M{
			"processingStatus": types.ProcessingStatusProcessing,
			"updatedAt":        time.Now(),
		},
	}
	collection.UpdateOne(context.Background(), bson.M{"_id": simulation.ID}, update)

	// Get simulation directory for cometbft-log-etl
	simulationDir := utils.GetSimulationDir(simulation.UserID, simulation.ProjectID, simulation.ID)

	// Execute cometbft-log-etl with simulation ID
	cmd := exec.Command("cometbft-log-etl", "-dir", simulationDir, "-simulation", simulation.ID.Hex())
	err := cmd.Run()

	var processingResult types.ProcessingResult
	var status types.ProcessingStatus
	var simulationStatus types.SimulationStatus
	processingTime := time.Since(startTime).Milliseconds()

	if err != nil {
		// Processing failed
		status = types.ProcessingStatusFailed
		simulationStatus = types.SimulationStatusFailed
		processingResult = types.ProcessingResult{
			ProcessedFiles: 0,
			TotalFiles:     simulation.LogFileCount(),
			ProcessingTime: processingTime,
			ErrorMessage:   fmt.Sprintf("Parser execution failed: %v.", err),
			ProcessedAt:    time.Now(),
		}
	} else {
		// Processing succeeded
		status = types.ProcessingStatusCompleted
		simulationStatus = types.SimulationStatusProcessed
		processingResult = types.ProcessingResult{
			ProcessedFiles: simulation.LogFileCount(),
			TotalFiles:     simulation.LogFileCount(),
			ProcessingTime: processingTime,
			ProcessedAt:    time.Now(),
		}

		// Create processed directory for future output files
		_, dirErr := utils.EnsureProcessedDir(simulation.UserID, simulation.ProjectID, simulation.ID)
		if dirErr != nil {
			fmt.Printf("Warning: Failed to create processed directory: %v\n", dirErr)
		}
	}

	// Update simulation with final result
	finalUpdate := bson.M{
		"$set": bson.M{
			"status":           simulationStatus,
			"processingStatus": status,
			"processingResult": processingResult,
			"updatedAt":        time.Now(),
		},
	}
	collection.UpdateOne(context.Background(), bson.M{"_id": simulation.ID}, finalUpdate)
}

// processSimulationAsync handles the async processing logic
func processSimulationAsync(collection *mongo.Collection, simulation types.Simulation) {
	// This function can be extended with additional processing logic
	// For now, it's just a placeholder for the async processing workflow
}
