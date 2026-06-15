package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"lingqu-dou-gate/internal/config"
	"lingqu-dou-gate/internal/handlers"
	"lingqu-dou-gate/internal/services"
)

func main() {
	config.Load()

	services.InitDB()
	defer services.CloseDB()

	services.InitMQTT()
	defer services.CloseMQTT()

	handler := handlers.NewHandler()

	r := gin.Default()

	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	api := r.Group("/api")
	{
		api.GET("/gates", handler.GetGates)
		api.GET("/gates/:id", handler.GetGate)

		api.GET("/sensors/:gateId", handler.GetSensorData)
		api.GET("/sensors/:gateId/history", handler.GetSensorHistory)
		api.POST("/sensors", handler.PostSensorData)

		api.POST("/simulate", handler.SimulatePassage)
		api.GET("/simulation/:gateId", handler.GetSimulationData)

		api.POST("/optimize", handler.OptimizeSchedule)

		api.GET("/alerts", handler.GetAlerts)
		api.POST("/alerts/:id/resolve", handler.ResolveAlert)
		api.POST("/alerts/test", handler.TestAlert)
	}

	addr := config.AppConfig.Server.Host + ":" + config.AppConfig.Server.Port
	log.Printf("Server starting on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
