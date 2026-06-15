package hydraulic_sim

import (
	"log"
	"math"
	"sync"

	"lingqu-dou-gate/internal/config"
	"lingqu-dou-gate/internal/models"
)

type FlowRegime int

const (
	FreeFlow FlowRegime = iota
	TransitionalFlow
	SubmergedFlow
	WeirFlow
)

func (f FlowRegime) String() string {
	switch f {
	case FreeFlow:
		return "free"
	case TransitionalFlow:
		return "transitional"
	case SubmergedFlow:
		return "submerged"
	case WeirFlow:
		return "weir"
	default:
		return "unknown"
	}
}

type SimulateRequest struct {
	Gate           models.DouGate
	WaterLevelUp   float64
	WaterLevelDown float64
	GateOpening    float64
	Direction      string
	ReplyChan      chan *SimulateResult
}

type SimulateResult struct {
	FillTime         float64                  `json:"fill_time"`
	DrainTime        float64                  `json:"drain_time"`
	WaterLevelCurve  []models.WaterLevelPoint `json:"water_level_curve"`
	FlowRateCurve    []models.FlowRatePoint   `json:"flow_rate_curve"`
	MaxFlowRate      float64                  `json:"max_flow_rate"`
	AvgFlowRate      float64                  `json:"avg_flow_rate"`
	TotalWaterVolume float64                  `json:"total_water_volume"`
	Regime           FlowRegime               `json:"regime,omitempty"`
	Error            error                    `json:"-"`
}

type HydraulicSimulator struct {
	mu          sync.RWMutex
	running     bool
	requestChan chan SimulateRequest
	stopChan    chan struct{}
	wg          sync.WaitGroup
	params      config.HydraulicJSONConfig
	workerCount int
}

func NewHydraulicSimulator(workerCount int) *HydraulicSimulator {
	if workerCount <= 0 {
		workerCount = 2
	}
	return &HydraulicSimulator{
		requestChan: make(chan SimulateRequest, 50),
		stopChan:    make(chan struct{}),
		params:      config.AppConfig.HydraulicJSON,
		workerCount: workerCount,
	}
}

func (h *HydraulicSimulator) RequestChannel() chan<- SimulateRequest {
	return h.requestChan
}

func (h *HydraulicSimulator) Submit(req SimulateRequest) {
	select {
	case h.requestChan <- req:
	default:
		if req.ReplyChan != nil {
			req.ReplyChan <- &SimulateResult{Error: &SimulatorError{Message: "simulator queue full"}}
		}
	}
}

type SimulatorError struct {
	Message string
}

func (e *SimulatorError) Error() string {
	return e.Message
}

func (h *HydraulicSimulator) Start() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.running {
		return
	}
	h.running = true
	h.params = config.AppConfig.HydraulicJSON

	for i := 0; i < h.workerCount; i++ {
		h.wg.Add(1)
		go h.worker(i)
	}

	log.Printf("Hydraulic simulator started with %d workers", h.workerCount)
}

func (h *HydraulicSimulator) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.running {
		return
	}
	h.running = false

	close(h.stopChan)
	h.wg.Wait()
	close(h.requestChan)

	log.Println("Hydraulic simulator stopped")
}

func (h *HydraulicSimulator) worker(id int) {
	defer h.wg.Done()

	for {
		select {
		case <-h.stopChan:
			return
		case req, ok := <-h.requestChan:
			if !ok {
				return
			}
			result := h.simulatePassage(req)
			if req.ReplyChan != nil {
				select {
				case req.ReplyChan <- result:
				default:
				}
			}
		}
	}
}

func (h *HydraulicSimulator) simulatePassage(req SimulateRequest) *SimulateResult {
	gate := req.Gate
	waterLevelUp := req.WaterLevelUp
	waterLevelDown := req.WaterLevelDown
	gateOpening := req.GateOpening
	direction := req.Direction

	if waterLevelUp <= 0 {
		waterLevelUp = gate.MaxWaterLevelUp
	}
	if waterLevelDown <= 0 {
		waterLevelDown = gate.MinWaterLevelDown
	}
	if gateOpening <= 0 {
		gateOpening = 1.0
	}
	if direction == "" {
		direction = "upstream"
	}

	chamberArea := gate.ChamberLength * gate.ChamberWidth
	var fillTime, drainTime float64
	var levelCurve []models.WaterLevelPoint
	var flowCurve []models.FlowRatePoint

	if direction == "upstream" {
		fillTime, levelCurve, flowCurve = h.calculateFillTime(
			waterLevelUp, waterLevelDown, gateOpening, chamberArea,
		)
		drainTime = 0
	} else {
		drainTime, levelCurve, flowCurve = h.calculateDrainTime(
			waterLevelUp, waterLevelDown, gateOpening, chamberArea,
		)
		fillTime = 0
	}

	var maxFlowRate, avgFlowRate, totalFlow float64
	for _, fp := range flowCurve {
		if fp.FlowRate > maxFlowRate {
			maxFlowRate = fp.FlowRate
		}
		totalFlow += fp.FlowRate
	}
	if len(flowCurve) > 0 {
		avgFlowRate = totalFlow / float64(len(flowCurve))
	}

	totalVolume := math.Abs(waterLevelUp-waterLevelDown) * chamberArea

	return &SimulateResult{
		FillTime:         fillTime,
		DrainTime:        drainTime,
		WaterLevelCurve:  levelCurve,
		FlowRateCurve:    flowCurve,
		MaxFlowRate:      maxFlowRate,
		AvgFlowRate:      avgFlowRate,
		TotalWaterVolume: totalVolume,
	}
}

func (h *HydraulicSimulator) calculateFillTime(
	targetLevelUp, initialChamberLevel, gateOpening, chamberArea float64,
) (float64, []models.WaterLevelPoint, []models.FlowRatePoint) {
	return h.calculateLevelChange(
		targetLevelUp, initialChamberLevel, gateOpening, chamberArea, true,
	)
}

func (h *HydraulicSimulator) calculateDrainTime(
	initialChamberLevel, targetLevelDown, gateOpening, chamberArea float64,
) (float64, []models.WaterLevelPoint, []models.FlowRatePoint) {
	return h.calculateLevelChange(
		targetLevelDown, initialChamberLevel, gateOpening, chamberArea, false,
	)
}

func (h *HydraulicSimulator) calculateLevelChange(
	targetLevel, initialLevel, opening, area float64, isFilling bool,
) (float64, []models.WaterLevelPoint, []models.FlowRatePoint) {
	dt := h.params.Simulation.TimeStep
	var totalTime float64
	var levelPoints []models.WaterLevelPoint
	var flowPoints []models.FlowRatePoint

	currentLevel := initialLevel
	levelPoints = append(levelPoints, models.WaterLevelPoint{Time: 0, WaterLevel: currentLevel})

	maxIterations := h.params.Simulation.MaxIterations
	iterations := 0

	targetReached := func() bool {
		if isFilling {
			return currentLevel >= targetLevel-0.0005
		}
		return currentLevel <= targetLevel+0.0005
	}

	for !targetReached() && iterations < maxIterations {
		var flowRate float64
		if isFilling {
			flowRate = h.calculateOrificeFlow(targetLevel, currentLevel, opening)
		} else {
			flowRate = h.calculateOrificeFlow(currentLevel, targetLevel, opening)
		}
		flowPoints = append(flowPoints, models.FlowRatePoint{Time: totalTime, FlowRate: flowRate})

		dtAdaptive := dt
		if flowRate > 0 {
			levelDiff := math.Abs(targetLevel - currentLevel)
			maxDH := levelDiff * h.params.Simulation.AdaptiveStepRatio
			theoreticalDT := maxDH * area / flowRate
			if theoreticalDT < dtAdaptive && theoreticalDT > h.params.Simulation.MinDT {
				dtAdaptive = theoreticalDT
			}
		}

		dv := flowRate * dtAdaptive
		dh := dv / area

		if isFilling {
			currentLevel += dh
		} else {
			currentLevel -= dh
		}
		totalTime += dtAdaptive
		iterations++

		levelThreshold := math.Abs(targetLevel-initialLevel) * 0.1
		if math.Mod(float64(iterations), 5) == 0 || math.Abs(dh) > levelThreshold || targetReached() {
			levelPoints = append(levelPoints, models.WaterLevelPoint{
				Time:       totalTime,
				WaterLevel: currentLevel,
			})
		}
	}

	if len(levelPoints) == 0 || levelPoints[len(levelPoints)-1].Time != totalTime {
		levelPoints = append(levelPoints, models.WaterLevelPoint{
			Time:       totalTime,
			WaterLevel: currentLevel,
		})
	}

	if len(flowPoints) == 0 || flowPoints[len(flowPoints)-1].Time != totalTime {
		flowPoints = append(flowPoints, models.FlowRatePoint{
			Time:     totalTime,
			FlowRate: 0,
		})
	}

	return totalTime, levelPoints, flowPoints
}

func (h *HydraulicSimulator) calculateOrificeFlow(
	waterLevelUp, waterLevelDown, gateOpening float64,
) float64 {
	headDiff := waterLevelUp - waterLevelDown
	if headDiff <= 0 {
		return 0
	}

	gateWidth := 6.0
	gateHeight := 4.5
	openingHeight := gateOpening * gateHeight
	if openingHeight <= 0 {
		return 0
	}

	relativeOpening := openingHeight / waterLevelUp
	if relativeOpening > 1 {
		relativeOpening = 1
	}

	Cc := h.calculateContractionCoefficient(relativeOpening)
	effectiveHeight := Cc * openingHeight

	regime := h.classifyFlowRegime(waterLevelUp, waterLevelDown, openingHeight, relativeOpening)

	var flowRate float64
	switch regime {
	case FreeFlow:
		flowRate = h.calculateFreeFlowRate(waterLevelUp, openingHeight, relativeOpening, gateWidth)
	case SubmergedFlow:
		submergenceRatio := 0.0
		vcDepth := effectiveHeight
		if headDiff > 0 {
			submergenceRatio = waterLevelDown / (waterLevelUp - vcDepth)
		}
		flowRate = h.calculateSubmergedFlowRate(
			waterLevelUp, waterLevelDown, openingHeight, relativeOpening, submergenceRatio, gateWidth,
		)
	case TransitionalFlow:
		freeFlow := h.calculateFreeFlowRate(waterLevelUp, openingHeight, relativeOpening, gateWidth)
		vcDepth := effectiveHeight
		submergenceRatio := waterLevelDown / (waterLevelUp - vcDepth)
		submergedFlow := h.calculateSubmergedFlowRate(
			waterLevelUp, waterLevelDown, openingHeight, relativeOpening, submergenceRatio, gateWidth,
		)
		transitionStart := h.params.FlowRegime.FreeFlowThreshold
		transitionEnd := h.params.FlowRegime.SubmergedThreshold
		transitionWidth := transitionEnd - transitionStart
		position := (submergenceRatio - transitionStart) / transitionWidth
		smoothStep := position * position * (3 - 2*position)
		flowRate = freeFlow*(1-smoothStep) + submergedFlow*smoothStep
	case WeirFlow:
		flowRate = h.calculateWeirFlowRate(waterLevelUp, waterLevelDown, headDiff, gateWidth)
	default:
		flowRate = h.calculateFreeFlowRate(waterLevelUp, openingHeight, relativeOpening, gateWidth)
	}

	return math.Max(0, flowRate)
}

func (h *HydraulicSimulator) calculateContractionCoefficient(relativeOpening float64) float64 {
	if relativeOpening <= 0 {
		return 0.6
	}
	if relativeOpening >= 1 {
		return 1.0
	}
	coeff := h.params.ContractionCoeff
	Cc := coeff.Base + coeff.LinearTerm*relativeOpening + coeff.QuadraticTerm*relativeOpening*relativeOpening
	return Cc
}

func (h *HydraulicSimulator) classifyFlowRegime(
	waterLevelUp, waterLevelDown, openingHeight, relativeOpening float64,
) FlowRegime {
	headTotal := waterLevelUp - waterLevelDown
	Cc := h.calculateContractionCoefficient(relativeOpening)
	vcDepth := Cc * openingHeight

	submergenceRatio := 0.0
	if headTotal > 0 {
		submergenceRatio = waterLevelDown / (waterLevelUp - vcDepth)
	}

	regimeCfg := h.params.FlowRegime
	if relativeOpening > regimeCfg.WeirRelativeOpening && headTotal/relativeOpening < regimeCfg.WeirHeadRatio {
		return WeirFlow
	} else if submergenceRatio < regimeCfg.FreeFlowThreshold {
		return FreeFlow
	} else if submergenceRatio > regimeCfg.SubmergedThreshold {
		return SubmergedFlow
	} else {
		return TransitionalFlow
	}
}

func (h *HydraulicSimulator) calculateFreeFlowRate(
	waterLevelUp, openingHeight, relativeOpening, gateWidth float64,
) float64 {
	Cc := h.calculateContractionCoefficient(relativeOpening)
	effectiveHeight := Cc * openingHeight

	var Cd float64
	if relativeOpening < 0.1 {
		Cd = 0.60
	} else if relativeOpening < 0.5 {
		Cd = 0.60 - 0.03*(relativeOpening-0.1)/0.4
	} else if relativeOpening < 0.8 {
		Cd = 0.57 + 0.06*(relativeOpening-0.5)/0.3
	} else {
		Cd = 0.63
	}

	headUp := waterLevelUp - effectiveHeight
	if headUp <= 0 {
		headUp = waterLevelUp * 0.7
	}

	gravity := h.params.Gravity
	flowRate := Cd * effectiveHeight * gateWidth * math.Sqrt(2*gravity*headUp)

	return math.Max(0, flowRate)
}

func (h *HydraulicSimulator) calculateSubmergedFlowRate(
	waterLevelUp, waterLevelDown, openingHeight, relativeOpening, submergenceRatio, gateWidth float64,
) float64 {
	Cc := h.calculateContractionCoefficient(relativeOpening)
	effectiveHeight := Cc * openingHeight
	headDiff := waterLevelUp - waterLevelDown

	if headDiff <= 0 {
		return 0
	}

	eta := submergenceRatio
	sigma := 1.0
	fullySub := h.params.FlowRegime.FullySubmerged
	submergedThr := h.params.FlowRegime.SubmergedThreshold
	if eta >= fullySub {
		sigma = 0.0
	} else if eta > submergedThr {
		transWidth := fullySub - submergedThr
		sigma = math.Sqrt((fullySub - eta) / transWidth)
	}

	Cd := h.params.DefaultCd * h.params.SubmergedCoeff
	gravity := h.params.Gravity
	flowRate := Cd * sigma * effectiveHeight * gateWidth * math.Sqrt(2*gravity*headDiff)

	return math.Max(0, flowRate)
}

func (h *HydraulicSimulator) calculateWeirFlowRate(
	waterLevelUp, waterLevelDown, headDiff, gateWidth float64,
) float64 {
	if headDiff <= 0 {
		return 0
	}
	submergenceRatio := waterLevelDown / waterLevelUp
	sigma := 1.0
	if submergenceRatio > 0.8 {
		sigma = 1.0 - (submergenceRatio-0.8)/0.2
		if sigma < 0 {
			sigma = 0
		}
	}
	Cw := h.params.WeirCoeff
	flowRate := sigma * Cw * gateWidth * math.Pow(headDiff, 1.5)
	return math.Max(0, flowRate)
}

func (h *HydraulicSimulator) ReloadConfig() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.params = config.AppConfig.HydraulicJSON
}
