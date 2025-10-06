package handlers

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/bft-labs/cometbft-analyzer-backend/types"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// validateUserInput performs additional custom validation
func validateUserInput(req *types.CreateUserRequest) error {
	// Check for reserved usernames
	reservedUsernames := []string{"admin", "root", "system", "api", "www", "mail", "ftp"}
	for _, reserved := range reservedUsernames {
		if strings.ToLower(req.Username) == reserved {
			return errors.New("username is reserved")
		}
	}

	// Additional email validation beyond the built-in validator
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(req.Email) {
		return errors.New("invalid email format")
	}

	// Check email domain is not blacklisted
	blacklistedDomains := []string{"example.com", "test.com", "invalid.com"}
	emailParts := strings.Split(req.Email, "@")
	if len(emailParts) == 2 {
		domain := strings.ToLower(emailParts[1])
		for _, blacklisted := range blacklistedDomains {
			if domain == blacklisted {
				return errors.New("email domain is not allowed")
			}
		}
	}

	return nil
}

// CreateUserHandler creates a new user
func CreateUserHandler(collection *mongo.Collection) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.CreateUserRequest

		if err := c.ShouldBindJSON(&req); err != nil {
			var errorMessages []string
			if validationErrors, ok := err.(validator.ValidationErrors); ok {
				for _, e := range validationErrors {
					switch e.Tag() {
					case "required":
						errorMessages = append(errorMessages, e.Field()+" is required")
					case "email":
						errorMessages = append(errorMessages, "Invalid email format")
					case "min":
						errorMessages = append(errorMessages, e.Field()+" must be at least "+e.Param()+" characters")
					case "max":
						errorMessages = append(errorMessages, e.Field()+" must be at most "+e.Param()+" characters")
					case "alphanum":
						errorMessages = append(errorMessages, e.Field()+" must contain only alphanumeric characters")
					default:
						errorMessages = append(errorMessages, e.Field()+" is invalid")
					}
				}
			} else {
				errorMessages = append(errorMessages, "Invalid JSON format")
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed", "details": errorMessages})
			return
		}

		// Additional custom validation
		if err := validateUserInput(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Check if user already exists
		var existingUser types.User
		err := collection.FindOne(context.Background(), bson.M{
			"$or": []bson.M{
				{"username": req.Username},
				{"email": req.Email},
			},
		}).Decode(&existingUser)

		if err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "User with this username or email already exists"})
			return
		} else if err != mongo.ErrNoDocuments {
			// Log the actual error but don't expose it to the client
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}

		user := types.User{
			Username:  req.Username,
			Email:     req.Email,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		result, err := collection.InsertOne(context.Background(), user)
		if err != nil {
			// Log the actual error but don't expose it to the client
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}

		user.ID = result.InsertedID.(primitive.ObjectID)
		c.JSON(http.StatusCreated, user)
	}
}

// GetUserHandler retrieves a user by ID
func GetUserHandler(collection *mongo.Collection) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.Param("userId")
		objectID, err := primitive.ObjectIDFromHex(userID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
			return
		}

		var user types.User
		err = collection.FindOne(context.Background(), bson.M{"_id": objectID}).Decode(&user)
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}

		c.JSON(http.StatusOK, user)
	}
}

// GetUsersHandler retrieves all users
func GetUsersHandler(collection *mongo.Collection) gin.HandlerFunc {
	return func(c *gin.Context) {
		cursor, err := collection.Find(context.Background(), bson.M{})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}
		defer cursor.Close(context.Background())

		var users []types.User
		if err := cursor.All(context.Background(), &users); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode users"})
			return
		}

		if users == nil {
			users = []types.User{}
		}

		c.JSON(http.StatusOK, users)
	}
}

// DeleteUserHandler deletes a user by ID
func DeleteUserHandler(collection *mongo.Collection) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.Param("userId")
		objectID, err := primitive.ObjectIDFromHex(userID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
			return
		}

		result, err := collection.DeleteOne(context.Background(), bson.M{"_id": objectID})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}

		if result.DeletedCount == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
	}
}
