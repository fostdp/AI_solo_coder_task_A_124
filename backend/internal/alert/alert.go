package alert

import (
	"log"
	"time"

	"lingqu-dou-gate/internal/models"
	"lingqu-dou-gate/internal/services"
)

type AlertManager struct {
	waterLevelThresholdUp   float64
	waterLevelThresholdDown float64
	passageTimeThreshold    float64
	flowRateThreshold       float64
}

func NewAlertManager() *AlertManager {
	return &AlertManager{
		waterLevelThresholdUp:   2.0,
		waterLevelThresholdDown: 0.5,
		passageTimeThreshold:    1800.0,
		flowRateThreshold:       50.0,
	}
}

func (am *AlertManager) CheckSensorData(gate models.DouGate, data models.SensorData) []models.Alert {
	var alerts []models.Alert

	if data.WaterLevelUp > gate.MaxWaterLevelUp {
		alerts = append(alerts, models.Alert{
			Time:      data.Time,
			GateID:    gate.ID,
			AlertType: "water_level_up_high",
			Severity:  "critical",
			Message:   "上游水位超过最高警戒值",
		})
	}

	if data.WaterLevelUp < gate.MinWaterLevelUp {
		alerts = append(alerts, models.Alert{
			Time:      data.Time,
			GateID:    gate.ID,
			AlertType: "water_level_up_low",
			Severity:  "warning",
			Message:   "上游水位低于最低警戒值",
		})
	}

	if data.WaterLevelDown > gate.MaxWaterLevelDown {
		alerts = append(alerts, models.Alert{
			Time:      data.Time,
			GateID:    gate.ID,
			AlertType: "water_level_down_high",
			Severity:  "warning",
			Message:   "下游水位超过最高警戒值",
		})
	}

	if data.WaterLevelDown < gate.MinWaterLevelDown {
		alerts = append(alerts, models.Alert{
			Time:      data.Time,
			GateID:    gate.ID,
			AlertType: "water_level_down_low",
			Severity:  "critical",
			Message:   "下游水位低于最低警戒值",
		})
	}

	levelDiff := data.WaterLevelUp - data.WaterLevelDown
	if levelDiff > am.waterLevelThresholdUp {
		alerts = append(alerts, models.Alert{
			Time:      data.Time,
			GateID:    gate.ID,
			AlertType: "water_level_diff_high",
			Severity:  "warning",
			Message:   "上下游水位差过大",
		})
	}

	if data.PassageTime > am.passageTimeThreshold {
		alerts = append(alerts, models.Alert{
			Time:      data.Time,
			GateID:    gate.ID,
			AlertType: "passage_time_exceeded",
			Severity:  "warning",
			Message:   "船舶通行时间超限",
		})
	}

	if data.GateOpening > 1.0 || data.GateOpening < 0 {
		alerts = append(alerts, models.Alert{
			Time:      data.Time,
			GateID:    gate.ID,
			AlertType: "gate_opening_abnormal",
			Severity:  "critical",
			Message:   "闸门开度异常",
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

func (am *AlertManager) ProcessAlerts(alerts []models.Alert) {
	for _, alert := range alerts {
		am.saveAlert(alert)
		am.publishAlert(alert)
	}
}

func (am *AlertManager) saveAlert(alert models.Alert) {
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

func (am *AlertManager) publishAlert(alert models.Alert) {
	if err := services.PublishAlert(alert); err != nil {
		log.Printf("Failed to publish alert to MQTT: %v", err)
	}
}

func (am *AlertManager) GetUnresolvedAlerts(gateID uint) ([]models.Alert, error) {
	db := services.GetDB()
	if db == nil {
		return nil, nil
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

func (am *AlertManager) ResolveAlert(alertID uint) error {
	db := services.GetDB()
	if db == nil {
		return nil
	}

	now := time.Now()
	return db.Model(&models.Alert{}).Where("id = ?", alertID).Updates(map[string]interface{}{
		"resolved":    true,
		"resolved_at": now,
	}).Error
}
