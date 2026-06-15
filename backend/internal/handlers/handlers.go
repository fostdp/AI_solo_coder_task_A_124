package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"lingqu-dou-gate/internal/middleware"
	"lingqu-dou-gate/internal/models"
	"lingqu-dou-gate/internal/modules/alarm_mqtt"
	"lingqu-dou-gate/internal/modules/dtu_receiver"
	"lingqu-dou-gate/internal/modules/hydraulic_sim"
	"lingqu-dou-gate/internal/modules/scheduler_ga"
	"lingqu-dou-gate/internal/services"
)

type Handler struct {
	sensorService *services.SensorService
	dtuReceiver   *dtu_receiver.DTUReceiver
	hydraulicSim  *hydraulic_sim.HydraulicSimulator
	schedulerGA   *scheduler_ga.GAScheduler
	alarmMqtt     *alarm_mqtt.AlarmMqtt
	metrics       *middleware.MetricsCollector
}

func NewHandler(
	dtu *dtu_receiver.DTUReceiver,
	hydro *hydraulic_sim.HydraulicSimulator,
	sched *scheduler_ga.GAScheduler,
	alarm *alarm_mqtt.AlarmMqtt,
	metrics *middleware.MetricsCollector,
) *Handler {
	return &Handler{
		sensorService: services.NewSensorService(),
		dtuReceiver:   dtu,
		hydraulicSim:  hydro,
		schedulerGA:   sched,
		alarmMqtt:     alarm,
		metrics:       metrics,
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

	h.dtuReceiver.Submit(data)
	h.metrics.IncSensorDataReceived()

	c.JSON(http.StatusAccepted, gin.H{"status": "accepted", "data": data})
}

func (h *Handler) SimulatePassage(c *gin.Context) {
	var req struct {
		GateID         uint    `json:"gate_id"`
		WaterLevelUp   float64 `json:"water_level_up"`
		WaterLevelDown float64 `json:"water_level_down"`
		GateOpening    float64 `json:"gate_opening"`
		Direction      string  `json:"direction"`
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

	replyChan := make(chan *hydraulic_sim.SimulateResult, 1)
	simReq := hydraulic_sim.SimulateRequest{
		Gate:           *gate,
		WaterLevelUp:   req.WaterLevelUp,
		WaterLevelDown: req.WaterLevelDown,
		GateOpening:    req.GateOpening,
		Direction:      req.Direction,
		ReplyChan:      replyChan,
	}

	h.hydraulicSim.Submit(simReq)

	select {
	case result := <-replyChan:
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
			return
		}
		h.metrics.IncSimulation()
		c.JSON(http.StatusOK, gin.H{"data": result})
	case <-time.After(10 * time.Second):
		c.JSON(http.StatusRequestTimeout, gin.H{"error": "simulation timeout"})
	}
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
		if len(allGates) >= 5 {
			gates = allGates[:5]
		} else {
			gates = allGates
		}
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

	replyChan := make(chan *scheduler_ga.OptimizeResult, 1)
	optReq := scheduler_ga.OptimizeRequest{
		Gates:       gates,
		Ships:       req.Ships,
		PassageTime: passageTime,
		ReplyChan:   replyChan,
	}

	h.schedulerGA.Submit(optReq)

	select {
	case result := <-replyChan:
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
			return
		}
		h.metrics.IncOptimization(result.Generations, result.TotalWaitTime)
		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{
				"schedule":      result.Schedule,
				"total_wait_time": result.TotalWaitTime,
				"fitness":       result.Fitness,
				"generations":   result.Generations,
				"history_count": result.HistoryCount,
			},
		})
	case <-time.After(30 * time.Second):
		c.JSON(http.StatusRequestTimeout, gin.H{"error": "optimization timeout"})
	}
}

func (h *Handler) GetAlerts(c *gin.Context) {
	gateID := uint(0)
	if idStr := c.Query("gate_id"); idStr != "" {
		id, _ := strconv.Atoi(idStr)
		gateID = uint(id)
	}

	alerts, err := h.alarmMqtt.GetUnresolvedAlerts(gateID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": alerts})
}

func (h *Handler) ResolveAlert(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := h.alarmMqtt.ResolveAlert(uint(id)); err != nil {
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
	h.alarmMqtt.ProcessAlertsManual(alerts)
	h.metrics.ProcessAlertsBridge(alerts)

	c.JSON(http.StatusOK, gin.H{"status": "alert sent", "data": testAlert})
}

func (h *Handler) GetSimulationData(c *gin.Context) {
	gateID, _ := strconv.Atoi(c.Param("gateId"))
	gate, err := h.sensorService.GetGateByID(uint(gateID))
	if gate == nil || err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Gate not found"})
		return
	}

	replyChan := make(chan *hydraulic_sim.SimulateResult, 1)
	simReq := hydraulic_sim.SimulateRequest{
		Gate:           *gate,
		WaterLevelUp:   gate.MaxWaterLevelUp,
		WaterLevelDown: gate.MinWaterLevelDown,
		GateOpening:    0.8,
		Direction:      "upstream",
		ReplyChan:      replyChan,
	}
	h.hydraulicSim.Submit(simReq)

	var simResult *hydraulic_sim.SimulateResult
	select {
	case simResult = <-replyChan:
		if simResult.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": simResult.Error.Error()})
			return
		}
	case <-time.After(15 * time.Second):
		c.JSON(http.StatusRequestTimeout, gin.H{"error": "simulation timeout"})
		return
	}

	passageTime := simResult.FillTime + 300
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
	optReplyChan := make(chan *scheduler_ga.OptimizeResult, 1)
	optReq := scheduler_ga.OptimizeRequest{
		Gates:       gates,
		Ships:       scheduleShips,
		PassageTime: passageTime,
		ReplyChan:   optReplyChan,
	}
	h.schedulerGA.Submit(optReq)

	var scheduleItems []models.ScheduleItem
	select {
	case optResult := <-optReplyChan:
		if optResult.Error == nil {
			scheduleItems = optResult.Schedule
		}
	case <-time.After(20 * time.Second):
	}

	sensorData, _ := h.sensorService.GetLatestSensorData(uint(gateID))
	alerts, _ := h.alarmMqtt.GetUnresolvedAlerts(uint(gateID))

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"gate":        gate,
			"sensor_data": sensorData,
			"simulation":  simResult,
			"schedule":    scheduleItems,
			"alerts":      alerts,
		},
	})
}
