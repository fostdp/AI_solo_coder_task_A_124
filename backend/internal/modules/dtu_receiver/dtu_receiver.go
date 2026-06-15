package dtu_receiver

import (
	"fmt"
	"log"
	"sync"
	"time"

	"lingqu-dou-gate/internal/models"
	"lingqu-dou-gate/internal/services"
)

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error on %s: %s", e.Field, e.Message)
}

type DTUReceiver struct {
	mu                sync.RWMutex
	running           bool
	dataInChan        chan models.SensorData
	validatedOutChan  chan models.SensorData
	gateCache         map[uint]*models.DouGate
	stopChan          chan struct{}
	wg                sync.WaitGroup
	workerCount       int
}

func NewDTUReceiver(workerCount int) *DTUReceiver {
	if workerCount <= 0 {
		workerCount = 2
	}
	return &DTUReceiver{
		dataInChan:       make(chan models.SensorData, 100),
		validatedOutChan: make(chan models.SensorData, 100),
		gateCache:        make(map[uint]*models.DouGate),
		stopChan:         make(chan struct{}),
		workerCount:      workerCount,
	}
}

func (d *DTUReceiver) ValidatedDataChannel() <-chan models.SensorData {
	return d.validatedOutChan
}

func (d *DTUReceiver) Submit(data models.SensorData) {
	select {
	case d.dataInChan <- data:
	default:
		log.Printf("DTU receiver channel full, dropping data for gate %d", data.GateID)
	}
}

func (d *DTUReceiver) Start() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.running {
		return
	}
	d.running = true

	for i := 0; i < d.workerCount; i++ {
		d.wg.Add(1)
		go d.worker(i)
	}

	log.Printf("DTU receiver started with %d workers", d.workerCount)
}

func (d *DTUReceiver) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.running {
		return
	}
	d.running = false

	close(d.stopChan)
	d.wg.Wait()
	close(d.dataInChan)
	close(d.validatedOutChan)

	log.Println("DTU receiver stopped")
}

func (d *DTUReceiver) worker(id int) {
	defer d.wg.Done()

	for {
		select {
		case <-d.stopChan:
			return
		case data, ok := <-d.dataInChan:
			if !ok {
				return
			}
			d.processData(id, data)
		}
	}
}

func (d *DTUReceiver) processData(workerID int, data models.SensorData) {
	if err := d.validateData(&data); err != nil {
		log.Printf("[DTU-%d] Validation failed for gate %d: %v", workerID, data.GateID, err)
		return
	}

	if data.Time.IsZero() {
		data.Time = time.Now()
	}

	db := services.GetDB()
	if db != nil {
		if err := db.Create(&data).Error; err != nil {
			log.Printf("[DTU-%d] Failed to save sensor data: %v", workerID, err)
		}
	}

	select {
	case d.validatedOutChan <- data:
	default:
		log.Printf("[DTU-%d] Output channel full, skipping broadcast for gate %d", workerID, data.GateID)
	}
}

func (d *DTUReceiver) validateData(data *models.SensorData) error {
	if data.GateID == 0 {
		return &ValidationError{Field: "gate_id", Message: "gate_id is required"}
	}

	if data.WaterLevelUp < 0 || data.WaterLevelUp > 100 {
		return &ValidationError{Field: "water_level_up", Message: "water_level_up out of valid range [0, 100]"}
	}

	if data.WaterLevelDown < 0 || data.WaterLevelDown > 100 {
		return &ValidationError{Field: "water_level_down", Message: "water_level_down out of valid range [0, 100]"}
	}

	if data.GateOpening < 0 || data.GateOpening > 1 {
		return &ValidationError{Field: "gate_opening", Message: "gate_opening must be in [0, 1]"}
	}

	if data.FlowRate < 0 {
		return &ValidationError{Field: "flow_rate", Message: "flow_rate cannot be negative"}
	}

	if data.WaterLevelUp <= data.WaterLevelDown {
		return &ValidationError{Field: "water_level", Message: "water_level_up must be greater than water_level_down"}
	}

	return nil
}

func (d *DTUReceiver) GetCachedGate(gateID uint) *models.DouGate {
	d.mu.RLock()
	gate, exists := d.gateCache[gateID]
	d.mu.RUnlock()

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

	d.mu.Lock()
	d.gateCache[gateID] = &g
	d.mu.Unlock()

	return &g
}
