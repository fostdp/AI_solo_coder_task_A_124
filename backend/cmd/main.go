package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"lingqu-dou-gate/internal/config"
	"lingqu-dou-gate/internal/handlers"
	"lingqu-dou-gate/internal/modules/alarm_mqtt"
	"lingqu-dou-gate/internal/modules/dtu_receiver"
	"lingqu-dou-gate/internal/modules/hydraulic_sim"
	"lingqu-dou-gate/internal/modules/scheduler_ga"
	"lingqu-dou-gate/internal/services"
)

func main() {
	config.Load()

	services.InitDB()
	defer services.CloseDB()

	services.InitMQTT()
	defer services.CloseMQTT()

	dtuReceiver := dtu_receiver.NewDTUReceiver(2)
	hydraulicSim := hydraulic_sim.NewHydraulicSimulator(2)
	schedulerGA := scheduler_ga.NewGAScheduler(2)
	alarmMqtt := alarm_mqtt.NewAlarmMqtt(dtuReceiver.ValidatedDataChannel(), 2)

	dtuReceiver.Start()
	hydraulicSim.Start()
	schedulerGA.Start()
	alarmMqtt.Start()

	defer func() {
		dtuReceiver.Stop()
		hydraulicSim.Stop()
		schedulerGA.Stop()
		alarmMqtt.Stop()
	}()

	handler := handlers.NewHandler(
		dtuReceiver,
		hydraulicSim,
		schedulerGA,
		alarmMqtt,
	)

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
	log.Printf("Modules: DTU=running, HydraulicSim=running, SchedulerGA=running, AlarmMQTT=running")

	go func() {
		if err := r.Run(addr); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down gracefully...")
}
