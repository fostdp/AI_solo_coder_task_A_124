package alarm_mqtt

import (
	"fmt"
	"log"
	"sync"
	"time"

	"lingqu-dou-gate/internal/models"
	"lingqu-dou-gate/internal/services"
)

type AlertThresholds struct {
	WaterLevelDiffMax   float64
	PassageTimeMax      float64
	FlowRateMax         float64
	WaterLevelUpMaxDev  float64
	WaterLevelDownMaxDev float64
}

type AlertConfig struct {
	Thresholds AlertThresholds
	MQTTPrefix string
}

type AlarmMqtt struct {
	mu             sync.RWMutex
	running        bool
	sensorDataChan <-chan models.SensorData
	alertOutChan   chan models.Alert
	stopChan       chan struct{}
	wg             sync.WaitGroup
	config         AlertConfig
	workerCount    int
	gateCache      map[uint]*models.DouGate
}

func NewAlarmMqtt(sensorDataChan <-chan models.SensorData, workerCount int) *AlarmMqtt {
	if workerCount <= 0 {
		workerCount = 1
	}
	return &AlarmMqtt{
		sensorDataChan: sensorDataChan,
		alertOutChan:   make(chan models.Alert, 100),
		stopChan:       make(chan struct{}),
		config: AlertConfig{
			Thresholds: AlertThresholds{
				WaterLevelDiffMax:  2.0,
				PassageTimeMax:     1800.0,
				FlowRateMax:        50.0,
			},
			MQTTPrefix: "lingqu/alerts",
		},
		workerCount: workerCount,
		gateCache:   make(map[uint]*models.DouGate),
	}
}

func (a *AlarmMqtt) AlertChannel() <-chan models.Alert {
	return a.alertOutChan
}

func (a *AlarmMqtt) SetConfig(cfg AlertConfig) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.config = cfg
}

func (a *AlarmMqtt) Start() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.running {
		return
	}
	a.running = true

	for i := 0; i < a.workerCount; i++ {
		a.wg.Add(1)
		go a.worker(i)
	}

	a.wg.Add(1)
	go a.mqttPublisher()

	log.Printf("Alarm-MQTT module started with %d workers", a.workerCount)
}

func (a *AlarmMqtt) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.running {
		return
	}
	a.running = false

	close(a.stopChan)
	a.wg.Wait()
	close(a.alertOutChan)

	log.Println("Alarm-MQTT module stopped")
}

func (a *AlarmMqtt) worker(id int) {
	defer a.wg.Done()

	for {
		select {
		case <-a.stopChan:
			return
		case data, ok := <-a.sensorDataChan:
			if !ok {
				return
			}
			a.processSensorData(id, data)
		}
	}
}

func (a *AlarmMqtt) processSensorData(workerID int, data models.SensorData) {
	gate := a.getGate(data.GateID)
	if gate == nil {
		return
	}

	alerts := a.evaluateAlerts(*gate, data)

	for _, alert := range alerts {
		a.saveAlert(alert)

		select {
		case a.alertOutChan <- alert:
		default:
			log.Printf("[ALARM-%d] Alert output channel full, dropping alert: %s", workerID, alert.AlertType)
		}
	}
}

func (a *AlarmMqtt) evaluateAlerts(gate models.DouGate, data models.SensorData) []models.Alert {
	var alerts []models.Alert
	thresholds := a.config.Thresholds

	if data.WaterLevelUp > gate.MaxWaterLevelUp {
		alerts = append(alerts, models.Alert{
			Time:      data.Time,
			GateID:    gate.ID,
			AlertType: "water_level_up_high",
			Severity:  "critical",
			Message:   fmt.Sprintf("上游水位%.2fm超过最高警戒值%.2fm", data.WaterLevelUp, gate.MaxWaterLevelUp),
		})
	}

	if data.WaterLevelUp < gate.MinWaterLevelUp {
		alerts = append(alerts, models.Alert{
			Time:      data.Time,
			GateID:    gate.ID,
			AlertType: "water_level_up_low",
			Severity:  "warning",
			Message:   fmt.Sprintf("上游水位%.2fm低于最低警戒值%.2fm", data.WaterLevelUp, gate.MinWaterLevelUp),
		})
	}

	if data.WaterLevelDown > gate.MaxWaterLevelDown {
		alerts = append(alerts, models.Alert{
			Time:      data.Time,
			GateID:    gate.ID,
			AlertType: "water_level_down_high",
			Severity:  "warning",
			Message:   fmt.Sprintf("下游水位%.2fm超过最高警戒值%.2fm", data.WaterLevelDown, gate.MaxWaterLevelDown),
		})
	}

	if data.WaterLevelDown < gate.MinWaterLevelDown {
		alerts = append(alerts, models.Alert{
			Time:      data.Time,
			GateID:    gate.ID,
			AlertType: "water_level_down_low",
			Severity:  "critical",
			Message:   fmt.Sprintf("下游水位%.2fm低于最低警戒值%.2fm", data.WaterLevelDown, gate.MinWaterLevelDown),
		})
	}

	levelDiff := data.WaterLevelUp - data.WaterLevelDown
	if levelDiff > thresholds.WaterLevelDiffMax {
		alerts = append(alerts, models.Alert{
			Time:      data.Time,
			GateID:    gate.ID,
			AlertType: "water_level_diff_high",
			Severity:  "warning",
			Message:   fmt.Sprintf("上下游水位差%.2fm超过警戒值%.2fm", levelDiff, thresholds.WaterLevelDiffMax),
		})
	}

	if data.PassageTime > thresholds.PassageTimeMax {
		alerts = append(alerts, models.Alert{
			Time:      data.Time,
			GateID:    gate.ID,
			AlertType: "passage_time_exceeded",
			Severity:  "warning",
			Message:   fmt.Sprintf("船舶通行时间%.0fs超过警戒值%.0fs", data.PassageTime, thresholds.PassageTimeMax),
		})
	}

	if data.GateOpening > 1.0 || data.GateOpening < 0 {
		alerts = append(alerts, models.Alert{
			Time:      data.Time,
			GateID:    gate.ID,
			AlertType: "gate_opening_abnormal",
			Severity:  "critical",
			Message:   fmt.Sprintf("闸门开度%.2f异常", data.GateOpening),
		})
	}

	if data.FlowRate > thresholds.FlowRateMax {
		alerts = append(alerts, models.Alert{
			Time:      data.Time,
			GateID:    gate.ID,
			AlertType: "flow_rate_exceeded",
			Severity:  "warning",
			Message:   fmt.Sprintf("流量%.2fm³/s超过警戒值%.2f", data.FlowRate, thresholds.FlowRateMax),
		})
	}

	if data.Status == "fault" {
		alerts = append(alerts, models.Alert{
			Time:      data.Time,
			GateID:    gate.ID,
			AlertType: "sensor_fault",
			Severity:  "critical",
			Message:   "传感器故障",
		})
	}

	return alerts
}

func (a *AlarmMqtt) saveAlert(alert models.Alert) {
	db := services.GetDB()
	if db == nil {
		return
	}

	alert.Time = time.Now()
	alert.Resolved = false

	if err := db.Create(&alert).Error; err != nil {
		log.Printf("Failed to save alert: %v", err)
	}
}

func (a *AlarmMqtt) mqttPublisher() {
	defer a.wg.Done()

	for {
		select {
		case <-a.stopChan:
			return
		case alert, ok := <-a.alertOutChan:
			if !ok {
				return
			}
			a.publishToMQTT(alert)
		}
	}
}

func (a *AlarmMqtt) publishToMQTT(alert models.Alert) {
	if err := services.PublishAlert(alert); err != nil {
		log.Printf("Failed to publish alert to MQTT: %v", err)
	} else {
		log.Printf("Alert published: gate=%d type=%s severity=%s", alert.GateID, alert.AlertType, alert.Severity)
	}
}

func (a *AlarmMqtt) getGate(gateID uint) *models.DouGate {
	a.mu.RLock()
	gate, exists := a.gateCache[gateID]
	a.mu.RUnlock()

	if exists {
		return gate
	}

	db := services.GetDB()
	if db == nil {
		return nil
	}

	var g models.DouGate
	if err := db.First(&g, gateID).Error; err != nil {
		return nil
	}

	a.mu.Lock()
	a.gateCache[gateID] = &g
	a.mu.Unlock()

	return &g
}

func (a *AlarmMqtt) GetUnresolvedAlerts(gateID uint) ([]models.Alert, error) {
	db := services.GetDB()
	if db == nil {
		return nil, fmt.Errorf("database not available")
	}

	var alerts []models.Alert
	query := db.Where("resolved = ?", false)
	if gateID > 0 {
		query = query.Where("gate_id = ?", gateID)
	}

	if err := query.Order("time DESC").Find(&alerts).Error; err != nil {
		return nil, err
	}
	return alerts, nil
}

func (a *AlarmMqtt) ResolveAlert(alertID uint) error {
	db := services.GetDB()
	if db == nil {
		return fmt.Errorf("database not available")
	}

	now := time.Now()
	return db.Model(&models.Alert{}).Where("id = ?", alertID).Updates(map[string]interface{}{
		"resolved":    true,
		"resolved_at": now,
	}).Error
}

func (a *AlarmMqtt) ProcessAlertsManual(alerts []models.Alert) {
	for _, alert := range alerts {
		a.saveAlert(alert)
		a.publishToMQTT(alert)
	}
}
