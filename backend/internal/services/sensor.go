package services

import (
	"math"
	"time"

	"lingqu-dou-gate/internal/models"
)

type SensorService struct{}

func NewSensorService() *SensorService {
	return &SensorService{}
}

func (s *SensorService) SaveSensorData(data models.SensorData) error {
	db := GetDB()
	if db == nil {
		return nil
	}
	return db.Create(&data).Error
}

func (s *SensorService) GetLatestSensorData(gateID uint) (*models.SensorData, error) {
	db := GetDB()
	if db == nil {
		return &models.SensorData{
			WaterLevelUp:   7.5,
			WaterLevelDown: 3.5,
			GateOpening:    0.5,
			FlowRate:       25.0,
			Status:         "normal",
			Time:           time.Now(),
		}, nil
	}

	var data models.SensorData
	err := db.Where("gate_id = ?", gateID).Order("time DESC").First(&data).Error
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (s *SensorService) GetSensorDataHistory(gateID uint, startTime, endTime time.Time) ([]models.SensorData, error) {
	db := GetDB()
	if db == nil {
		var mockData []models.SensorData
		now := time.Now()
		for i := 0; i < 100; i++ {
			t := now.Add(-time.Duration(i*5) * time.Minute)
			mockData = append(mockData, models.SensorData{
				Time:           t,
				GateID:         gateID,
				WaterLevelUp:   7.0 + 0.5*math.Sin(float64(i)*0.1),
				WaterLevelDown: 3.0 + 0.3*math.Sin(float64(i)*0.15),
				GateOpening:    0.4 + 0.2*math.Sin(float64(i)*0.08),
				FlowRate:       20.0 + 10.0*math.Sin(float64(i)*0.12),
				Status:         "normal",
			})
		}
		return mockData, nil
	}

	var data []models.SensorData
	err := db.Where("gate_id = ? AND time BETWEEN ? AND ?", gateID, startTime, endTime).
		Order("time ASC").Find(&data).Error
	return data, err
}

func (s *SensorService) GetAllGates() ([]models.DouGate, error) {
	db := GetDB()
	if db == nil {
		var mockGates []models.DouGate
		for i := 1; i <= 36; i++ {
			mockGates = append(mockGates, models.DouGate{
				ID:                   uint(i),
				Name:                 "陡门" + intToString(i),
				Location:             "灵渠第" + intToString(i) + "座",
				GateWidth:            5.5 + 1.5*math.Mod(float64(i), 3)/3,
				GateHeight:           4.0 + 1.5*math.Mod(float64(i), 4)/4,
				MaxWaterLevelUp:      8.0 + 1.5*math.Mod(float64(i), 5)/5,
				MinWaterLevelUp:      3.5 + 1.0*math.Mod(float64(i), 3)/3,
				MaxWaterLevelDown:    4.5 + 1.5*math.Mod(float64(i), 4)/4,
				MinWaterLevelDown:    1.5 + 1.0*math.Mod(float64(i), 3)/3,
				ChamberLength:        50.0 + 30.0*math.Mod(float64(i), 6)/6,
				ChamberWidth:         8.0 + 4.0*math.Mod(float64(i), 5)/5,
				DischargeCoefficient: 0.6 + 0.1*math.Mod(float64(i), 3)/3,
				Status:               "active",
			})
		}
		return mockGates, nil
	}

	var gates []models.DouGate
	err := db.Find(&gates).Error
	return gates, err
}

func (s *SensorService) GetGateByID(id uint) (*models.DouGate, error) {
	db := GetDB()
	if db == nil {
		i := int(id)
		return &models.DouGate{
			ID:                   id,
			Name:                 "陡门" + intToString(i),
			Location:             "灵渠第" + intToString(i) + "座",
			GateWidth:            6.0,
			GateHeight:           4.5,
			MaxWaterLevelUp:      8.5,
			MinWaterLevelUp:      4.0,
			MaxWaterLevelDown:    5.0,
			MinWaterLevelDown:    2.0,
			ChamberLength:        60.0,
			ChamberWidth:         10.0,
			DischargeCoefficient: 0.63,
			Status:               "active",
		}, nil
	}

	var gate models.DouGate
	err := db.First(&gate, id).Error
	return &gate, err
}

func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []rune{}
	for n > 0 {
		digits = append([]rune{rune('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
