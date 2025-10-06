package main

import (
	"log"
	"os"

	"github.com/bft-labs/cometbft-analyzer-backend/db"
	"github.com/bft-labs/cometbft-analyzer-backend/handlers"
	"github.com/bft-labs/cometbft-analyzer-backend/middleware"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found or error loading .env file")
	}

	// Load MongoDB URI, default to localhost
	mongoURI := os.Getenv("MONGODB_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27017"
	}

	client, err := db.Connect(mongoURI)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}

	// User management collections
	usersColl := client.Database("consensus_visualizer").Collection("users")
	projectsColl := client.Database("consensus_visualizer").Collection("projects")
	simulationsColl := client.Database("consensus_visualizer").Collection("simulations")

	router := gin.Default()

	// Add security middleware
	router.Use(middleware.SecurityHeadersMiddleware())
	router.Use(middleware.CORSMiddleware())
	router.Use(middleware.RequestValidationMiddleware())

	// Add rate limiting (60 requests per minute, burst of 10)
	router.Use(middleware.RateLimitMiddleware(6000, 10))

	v1 := router.Group("/v1")
	{
		// User management endpoints
		v1.POST("/users", handlers.CreateUserHandler(usersColl))
		v1.GET("/users", handlers.GetUsersHandler(usersColl))
		v1.GET("/users/:userId", handlers.GetUserHandler(usersColl))
		v1.DELETE("/users/:userId", handlers.DeleteUserHandler(usersColl))

		// Project management endpoints
		v1.POST("/users/:userId/projects", handlers.CreateProjectHandler(projectsColl))
		v1.GET("/users/:userId/projects", handlers.GetProjectsByUserHandler(projectsColl))
		v1.GET("/projects/:projectId", handlers.GetProjectHandler(projectsColl))
		v1.PUT("/projects/:projectId", handlers.UpdateProjectHandler(projectsColl))
		v1.DELETE("/projects/:projectId", handlers.DeleteProjectHandler(projectsColl))

		// Simulation management endpoints
		v1.POST("/users/:userId/projects/:projectId/simulations", handlers.CreateSimulationHandler(simulationsColl))
		v1.GET("/users/:userId/simulations", handlers.GetSimulationsByUserHandler(simulationsColl))
		v1.GET("/projects/:projectId/simulations", handlers.GetSimulationsByProjectHandler(simulationsColl))
		v1.GET("/simulations/:id", handlers.GetSimulationHandler(simulationsColl))
		v1.PUT("/simulations/:id", handlers.UpdateSimulationHandler(simulationsColl))
		v1.DELETE("/simulations/:id", handlers.DeleteSimulationHandler(simulationsColl))
		v1.POST("/simulations/:id/upload", handlers.UploadLogFileHandler(simulationsColl))
		v1.POST("/simulations/:id/process", handlers.ProcessSimulationHandler(simulationsColl))

		// Simulation-specific metrics endpoints
		v1.GET("/simulations/:id/events", handlers.GetSimulationConsensusEventsHandler(client, simulationsColl))
		v1.GET("/simulations/:id/metrics/latency/votes", handlers.GetSimulationVoteLatenciesHandler(client, simulationsColl))
		v1.GET("/simulations/:id/metrics/latency/pairwise", handlers.GetSimulationPairLatencyHandler(client, simulationsColl))
		v1.GET("/simulations/:id/metrics/latency/timeseries", handlers.GetSimulationBlockLatencyTimeSeriesHandler(client, simulationsColl))
		v1.GET("/simulations/:id/metrics/latency/stats", handlers.GetSimulationLatencyStatsHandler(client, simulationsColl))
		v1.GET("/simulations/:id/metrics/messages/success_rate", handlers.GetSimulationMessageSuccessRateHandler(client, simulationsColl))
		v1.GET("/simulations/:id/metrics/latency/end_to_end", handlers.GetSimulationBlockEndToEndLatencyHandler(client, simulationsColl))
		v1.GET("/simulations/:id/metrics/vote/statistics", handlers.GetSimulationVoteStatisticsHandler(client, simulationsColl))
		v1.GET("/simulations/:id/metrics/network/latency/stats", handlers.GetSimulationNetworkLatencyStatsHandler(client, simulationsColl))
		v1.GET("/simulations/:id/metrics/network/latency/node-stats", handlers.GetSimulationNetworkLatencyNodeStatsHandler(client, simulationsColl))
		v1.GET("/simulations/:id/metrics/network/latency/overview", handlers.GetSimulationNetworkLatencyOverviewHandler(client, simulationsColl))
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to run server: %v", err)
	}
}
