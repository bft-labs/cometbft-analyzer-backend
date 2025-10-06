package utils

import (
	"fmt"
	"os"
	"path/filepath"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// GetSimulationDir returns the directory path for a specific simulation
func GetSimulationDir(userID, projectID, simulationID primitive.ObjectID) string {
	return filepath.Join("uploads",
		fmt.Sprintf("user_%s", userID.Hex()),
		fmt.Sprintf("project_%s", projectID.Hex()),
		fmt.Sprintf("simulation_%s", simulationID.Hex()))
}

// EnsureSimulationDir creates the simulation directory if it doesn't exist
func EnsureSimulationDir(userID, projectID, simulationID primitive.ObjectID) (string, error) {
	dir := GetSimulationDir(userID, projectID, simulationID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create simulation directory: %w", err)
	}
	return dir, nil
}

// GetProcessedDir returns the processed files directory for a simulation
func GetProcessedDir(userID, projectID, simulationID primitive.ObjectID) string {
	return filepath.Join(GetSimulationDir(userID, projectID, simulationID), "processed")
}

// EnsureProcessedDir creates the processed directory if it doesn't exist
func EnsureProcessedDir(userID, projectID, simulationID primitive.ObjectID) (string, error) {
	dir := GetProcessedDir(userID, projectID, simulationID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create processed directory: %w", err)
	}
	return dir, nil
}
