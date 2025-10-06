package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/bft-labs/cometbft-analyzer-backend/types"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// CreateProjectHandler creates a new project
func CreateProjectHandler(collection *mongo.Collection) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.Param("userId")
		userObjectID, err := primitive.ObjectIDFromHex(userID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
			return
		}

		var req types.CreateProjectRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		project := types.Project{
			Name:        req.Name,
			Description: req.Description,
			UserID:      userObjectID,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		result, err := collection.InsertOne(context.Background(), project)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create project"})
			return
		}

		project.ID = result.InsertedID.(primitive.ObjectID)
		c.JSON(http.StatusCreated, project)
	}
}

// GetProjectHandler retrieves a project by ID
func GetProjectHandler(collection *mongo.Collection) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("projectId")
		objectID, err := primitive.ObjectIDFromHex(projectID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
			return
		}

		var project types.Project
		err = collection.FindOne(context.Background(), bson.M{"_id": objectID}).Decode(&project)
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
			return
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}

		c.JSON(http.StatusOK, project)
	}
}

// GetProjectsByUserHandler retrieves all projects for a specific user
func GetProjectsByUserHandler(collection *mongo.Collection) gin.HandlerFunc {
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

		var projects []types.Project
		if err := cursor.All(context.Background(), &projects); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode projects"})
			return
		}

		if projects == nil {
			projects = []types.Project{}
		}

		c.JSON(http.StatusOK, projects)
	}
}

// UpdateProjectHandler updates a project by ID
func UpdateProjectHandler(collection *mongo.Collection) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("projectId")
		objectID, err := primitive.ObjectIDFromHex(projectID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
			return
		}

		var req types.UpdateProjectRequest
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
			c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
			return
		}

		var project types.Project
		err = collection.FindOne(context.Background(), bson.M{"_id": objectID}).Decode(&project)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve updated project"})
			return
		}

		c.JSON(http.StatusOK, project)
	}
}

// DeleteProjectHandler deletes a project by ID
func DeleteProjectHandler(collection *mongo.Collection) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("projectId")
		objectID, err := primitive.ObjectIDFromHex(projectID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
			return
		}

		result, err := collection.DeleteOne(context.Background(), bson.M{"_id": objectID})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}

		if result.DeletedCount == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Project deleted successfully"})
	}
}
