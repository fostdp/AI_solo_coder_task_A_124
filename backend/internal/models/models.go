package models

import (
	"time"
)

type DouGate struct {
	ID                 uint      `gorm:"primaryKey" json:"id"`
	Name               string    `json:"name"`
	Location           string    `json:"location"`
	GateWidth          float64   `json:"gate_width"`
	GateHeight         float64   `json:"gate_height"`
	MaxWaterLevelUp    float64   `json:"max_water_level_up"`
	MinWaterLevelUp    float64   `json:"min_water_level_up"`
	MaxWaterLevelDown  float64   `json:"max_water_level_down"`
	MinWaterLevelDown  float64   `json:"min_water_level_down"`
	ChamberLength      float64   `json:"chamber_length"`
	ChamberWidth       float64   `json:"chamber_width"`
	DischargeCoefficient float64 `json:"discharge_coefficient"`
	Status             string    `json:"status"`
	CreatedAt          time.Time `json:"created_at"`
}

type SensorData struct {
	Time          time.Time `gorm:"primaryKey" json:"time"`
	GateID        uint      `gorm:"primaryKey" json:"gate_id"`
	WaterLevelUp  float64   `json:"water_level_up"`
	WaterLevelDown float64  `json:"water_level_down"`
	GateOpening   float64   `json:"gate_opening"`
	FlowRate      float64   `json:"flow_rate"`
	PassageTime   float64   `json:"passage_time"`
	Status        string    `json:"status"`
}

func (SensorData) TableName() string {
	return "sensor_data"
}

type Ship struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `json:"name"`
	Priority    int       `json:"priority"`
	Length      float64   `json:"length"`
	Width       float64   `json:"width"`
	Draft       float64   `json:"draft"`
	ArrivalTime time.Time `json:"arrival_time"`
	Direction   string    `json:"direction"`
	Status      string    `json:"status"`
}

type PassageRecord struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	ShipID     uint      `json:"ship_id"`
	GateID     uint      `json:"gate_id"`
	EntryTime  time.Time `json:"entry_time"`
	ExitTime   time.Time `json:"exit_time"`
	FillTime   float64   `json:"fill_time"`
	DrainTime  float64   `json:"drain_time"`
	TotalTime  float64   `json:"total_time"`
	WaitTime   float64   `json:"wait_time"`
	Status     string    `json:"status"`
}

type Alert struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Time       time.Time `json:"time"`
	GateID     uint      `json:"gate_id"`
	AlertType  string    `json:"alert_type"`
	Severity   string    `json:"severity"`
	Message    string    `json:"message"`
	Resolved   bool      `json:"resolved"`
	ResolvedAt time.Time `json:"resolved_at"`
}

type SchedulePlan struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	CreatedAt     time.Time `json:"created_at"`
	GateID        uint      `json:"gate_id"`
	Schedule      string    `gorm:"type:json" json:"schedule"`
	TotalWaitTime float64   `json:"total_wait_time"`
	Generation    int       `json:"generation"`
	Fitness       float64   `json:"fitness"`
}

type SimulationResult struct {
	FillTime           float64   `json:"fill_time"`
	DrainTime          float64   `json:"drain_time"`
	WaterLevelCurve    []WaterLevelPoint `json:"water_level_curve"`
	FlowRateCurve      []FlowRatePoint   `json:"flow_rate_curve"`
	MaxFlowRate        float64   `json:"max_flow_rate"`
	AvgFlowRate        float64   `json:"avg_flow_rate"`
	TotalWaterVolume   float64   `json:"total_water_volume"`
}

type WaterLevelPoint struct {
	Time       float64 `json:"time"`
	WaterLevel float64 `json:"water_level"`
}

type FlowRatePoint struct {
	Time     float64 `json:"time"`
	FlowRate float64 `json:"flow_rate"`
}

type ScheduleShip struct {
	ShipID     uint      `json:"ship_id"`
	ShipName   string    `json:"ship_name"`
	Priority   int       `json:"priority"`
	ArrivalTime time.Time `json:"arrival_time"`
	Direction  string    `json:"direction"`
}

type ScheduleItem struct {
	ShipID     uint      `json:"ship_id"`
	ShipName   string    `json:"ship_name"`
	StartTime  time.Time `json:"start_time"`
	EndTime    time.Time `json:"end_time"`
	WaitTime   float64   `json:"wait_time"`
	Priority   int       `json:"priority"`
	Direction  string    `json:"direction"`
}
