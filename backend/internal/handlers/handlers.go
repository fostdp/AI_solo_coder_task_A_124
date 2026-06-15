package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"lingqu-dou-gate/internal/alert"
	"lingqu-dou-gate/internal/models"
	"lingqu-dou-gate/internal/optimizer"
	"lingqu-dou-gate/internal/services"
	"lingqu-dou-gate/internal/simulation"
)

type Handler struct {
	sensorService *services.SensorService
	alertManager  *alert.AlertManager
}

func NewHandler() *Handler {
	return &Handler{
		sensorService: services.NewSensorService(),
		alertManager:  alert.NewAlertManager(),
	}
}

func (h *Handler) GetGates(c *gin.Context) {
	gates, err := h.sensorService.GetAllGates()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gates})
}

func (h *Handler) GetGate(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	gate, err := h.sensorService.GetGateByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Gate not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gate})
}

func (h *Handler) GetSensorData(c *gin.Context) {
	gateID, _ := strconv.Atoi(c.Param("gateId"))
	data, err := h.sensorService.GetLatestSensorData(uint(gateID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Sensor data not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

func (h *Handler) GetSensorHistory(c *gin.Context) {
	gateID, _ := strconv.Atoi(c.Param("gateId"))
	startStr := c.Query("start")
	endStr := c.Query("end")

	startTime := time.Now().Add(-24 * time.Hour)
	endTime := time.Now()

	if startStr != "" {
		if t, err := time.Parse(time.RFC3339, startStr); err == nil {
			startTime = t
		}
	}
	if endStr != "" {
		if t, err := time.Parse(time.RFC3339, endStr); err == nil {
			endTime = t
		}
	}

	data, err := h.sensorService.GetSensorDataHistory(uint(gateID), startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

func (h *Handler) PostSensorData(c *gin.Context) {
	var data models.SensorData
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data.Time.IsZero() {
		data.Time = time.Now()
	}

	gate, _ := h.sensorService.GetGateByID(data.GateID)

	if gate != nil {
		alerts := h.alertManager.CheckSensorData(*gate, data)
		if len(alerts) > 0 {
			go h.alertManager.ProcessAlerts(alerts)
		}
	}

	if err := h.sensorService.SaveSensorData(data); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"status": "success", "data": data})
}

func (h *Handler) SimulatePassage(c *gin.Context) {
	var req struct {
		GateID          uint    `json:"gate_id"`
		WaterLevelUp    float64 `json:"water_level_up"`
		WaterLevelDown  float64 `json:"water_level_down"`
		GateOpening     float64 `json:"gate_opening"`
		Direction       string  `json:"direction"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	gate, err := h.sensorService.GetGateByID(req.GateID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Gate not found"})
		return
	}

	if req.WaterLevelUp == 0 {
		req.WaterLevelUp = gate.MaxWaterLevelUp
	}
	if req.WaterLevelDown == 0 {
		req.WaterLevelDown = gate.MinWaterLevelDown
	}
	if req.GateOpening == 0 {
		req.GateOpening = 1.0
	}
	if req.Direction == "" {
		req.Direction = "upstream"
	}

	sim := simulation.NewHydroSimulator(*gate)
	result := sim.SimulateFullPassage(req.WaterLevelUp, req.WaterLevelDown, req.GateOpening, req.Direction)

	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (h *Handler) OptimizeSchedule(c *gin.Context) {
	var req struct {
		GateIDs []uint                `json:"gate_ids"`
		Ships   []models.ScheduleShip `json:"ships"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var gates []models.DouGate
	for _, id := range req.GateIDs {
		gate, err := h.sensorService.GetGateByID(id)
		if err == nil {
			gates = append(gates, *gate)
		}
	}

	if len(gates) == 0 {
		allGates, _ := h.sensorService.GetAllGates()
		gates = allGates[:5]
	}

	passageTime := 600.0
	if len(req.Ships) == 0 {
		now := time.Now()
		for i := 1; i <= 10; i++ {
			req.Ships = append(req.Ships, models.ScheduleShip{
				ShipID:      uint(i),
				ShipName:    "船舶" + strconv.Itoa(i),
				Priority:    (i % 5) + 1,
				ArrivalTime: now.Add(time.Duration(i*15) * time.Minute),
				Direction:   map[int]string{0: "upstream", 1: "downstream"}[i%2],
			})
		}
	}

	scheduler := optimizer.NewGAScheduler(gates, req.Ships, passageTime)
	bestSolution, history, generations := scheduler.Optimize()

	scheduleItems := scheduler.GetScheduleItems(bestSolution)

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"schedule":      scheduleItems,
			"total_wait_time": bestSolution.WaitTime,
			"fitness":       bestSolution.Fitness,
			"generations":   generations,
			"history_count": len(history),
		},
	})
}

func (h *Handler) GetAlerts(c *gin.Context) {
	gateID := uint(0)
	if idStr := c.Query("gate_id"); idStr != "" {
		id, _ := strconv.Atoi(idStr)
		gateID = uint(id)
	}

	alerts, err := h.alertManager.GetUnresolvedAlerts(gateID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": alerts})
}

func (h *Handler) ResolveAlert(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := h.alertManager.ResolveAlert(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func (h *Handler) TestAlert(c *gin.Context) {
	var testAlert models.Alert
	if err := c.ShouldBindJSON(&testAlert); err != nil {
		testAlert = models.Alert{
			GateID:    1,
			AlertType: "test",
			Severity:  "info",
			Message:   "测试告警消息",
		}
	}

	alerts := []models.Alert{testAlert}
	h.alertManager.ProcessAlerts(alerts)

	c.JSON(http.StatusOK, gin.H{"status": "alert sent", "data": testAlert})
}

func (h *Handler) GetSimulationData(c *gin.Context) {
	gateID, _ := strconv.Atoi(c.Param("gateId"))
	gate, _ := h.sensorService.GetGateByID(uint(gateID))
	if gate == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Gate not found"})
		return
	}

	sim := simulation.NewHydroSimulator(*gate)
	result := sim.SimulateFullPassage(gate.MaxWaterLevelUp, gate.MinWaterLevelDown, 0.8, "upstream")

	scheduleShips := []models.ScheduleShip{}
	now := time.Now()
	for i := 1; i <= 8; i++ {
		scheduleShips = append(scheduleShips, models.ScheduleShip{
			ShipID:      uint(i),
			ShipName:    "船舶" + strconv.Itoa(i),
			Priority:    (i % 3) + 1,
			ArrivalTime: now.Add(time.Duration(i*20) * time.Minute),
			Direction:   map[int]string{0: "upstream", 1: "downstream"}[i%2],
		})
	}

	gates := []models.DouGate{*gate}
	scheduler := optimizer.NewGAScheduler(gates, scheduleShips, result.FillTime+300)
	bestSolution, _, _ := scheduler.Optimize()
	scheduleItems := scheduler.GetScheduleItems(bestSolution)

	sensorData, _ := h.sensorService.GetLatestSensorData(uint(gateID))
	alerts, _ := h.alertManager.GetUnresolvedAlerts(uint(gateID))

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"gate":           gate,
			"sensor_data":    sensorData,
			"simulation":     result,
			"schedule":       scheduleItems,
			"alerts":         alerts,
		},
	})
}


