package simulation

import (
	"math"
	"sync"

	"lingqu-dou-gate/internal/config"
	"lingqu-dou-gate/internal/models"
)

type FlowRegime int

const (
	FreeFlow         FlowRegime = iota
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

type HydroSimulator struct {
	gravity       float64
	gate          models.DouGate
	kinematicVisc float64
	mu            sync.RWMutex
}

func NewHydroSimulator(gate models.DouGate) *HydroSimulator {
	return &HydroSimulator{
		gravity:       config.AppConfig.Hydro.Gravity,
		gate:          gate,
		kinematicVisc: 1e-6,
	}
}

func (h *HydroSimulator) calculateContractionCoefficient(relativeOpening float64) float64 {
	if relativeOpening <= 0 {
		return 0.6
	}
	if relativeOpening >= 1 {
		return 1.0
	}
	Cc := 0.615 + 0.105*relativeOpening - 0.02*relativeOpening*relativeOpening
	return Cc
}

func (h *HydroSimulator) calculateVenaContractaDepth(openingHeight, relativeOpening float64) float64 {
	Cc := h.calculateContractionCoefficient(relativeOpening)
	return Cc * openingHeight
}

func (h *HydroSimulator) calculateCriticalDepth(dischargePerUnitWidth float64) float64 {
	if dischargePerUnitWidth <= 0 {
		return 0
	}
	return math.Cbrt(dischargePerUnitWidth * dischargePerUnitWidth / h.gravity)
}

func (h *HydroSimulator) calculateFroudeNumber(velocity, depth float64) float64 {
	if depth <= 0 {
		return 0
	}
	return velocity / math.Sqrt(h.gravity*depth)
}

func (h *HydroSimulator) classifyFlowRegime(
	waterLevelUp, waterLevelDown, openingHeight, relativeOpening float64,
) (FlowRegime, float64, float64) {
	headTotal := waterLevelUp - waterLevelDown
	vcDepth := h.calculateVenaContractaDepth(openingHeight, relativeOpening)

	submergenceRatio := 0.0
	if headTotal > 0 {
		submergenceRatio = waterLevelDown / (waterLevelUp - vcDepth)
	}

	estimatedQ := h.gate.DischargeCoefficient * vcDepth * h.gate.GateWidth *
		math.Sqrt(2*h.gravity*(waterLevelUp-vcDepth))
	qPerUnit := 0.0
	if h.gate.GateWidth > 0 {
		qPerUnit = estimatedQ / h.gate.GateWidth
	}
	criticalDepth := h.calculateCriticalDepth(qPerUnit)

	var regime FlowRegime

	if relativeOpening > 0.75 && headTotal/relativeOpening < 0.3 {
		regime = WeirFlow
	} else if submergenceRatio < 0.67 {
		regime = FreeFlow
	} else if submergenceRatio > 0.88 {
		regime = SubmergedFlow
	} else {
		regime = TransitionalFlow
	}

	return regime, submergenceRatio, criticalDepth
}

func (h *HydroSimulator) calculateFreeFlowRate(
	waterLevelUp, openingHeight, relativeOpening float64,
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

	velocityCoefficient := 1.0
	Re := 0.0
	if h.kinematicVisc > 0 && openingHeight > 0 {
		velocity := Cd * math.Sqrt(2*h.gravity*headUp)
		Re = velocity * openingHeight / h.kinematicVisc
		if Re < 10000 {
			velocityCoefficient = 0.9 + 0.1*math.Log10(math.Max(Re, 1000))/4
		}
	}

	flowRate := Cd * velocityCoefficient * effectiveHeight * h.gate.GateWidth *
		math.Sqrt(2 * h.gravity * headUp)

	return math.Max(0, flowRate)
}

func (h *HydroSimulator) calculateSubmergedFlowRate(
	waterLevelUp, waterLevelDown, openingHeight, relativeOpening, submergenceRatio float64,
) float64 {
	Cc := h.calculateContractionCoefficient(relativeOpening)
	effectiveHeight := Cc * openingHeight
	headDiff := waterLevelUp - waterLevelDown

	if headDiff <= 0 {
		return 0
	}

	eta := submergenceRatio
	sigma := 1.0
	if eta >= 0.97 {
		sigma = 0.0
	} else if eta > 0.88 {
		sigma = math.Sqrt((0.97 - eta) / 0.09)
	} else {
		sigma = 1.0
	}

	Cd := h.gate.DischargeCoefficient * 0.92
	flowRate := Cd * sigma * effectiveHeight * h.gate.GateWidth *
		math.Sqrt(2 * h.gravity * headDiff)

	return math.Max(0, flowRate)
}

func (h *HydroSimulator) calculateTransitionalFlowRate(
	waterLevelUp, waterLevelDown, openingHeight, relativeOpening, submergenceRatio float64,
) (float64, FlowRegime) {
	freeFlow := h.calculateFreeFlowRate(waterLevelUp, openingHeight, relativeOpening)
	submergedFlow := h.calculateSubmergedFlowRate(
		waterLevelUp, waterLevelDown, openingHeight, relativeOpening, submergenceRatio,
	)

	transitionStart := 0.67
	transitionEnd := 0.88
	transitionWidth := transitionEnd - transitionStart
	position := (submergenceRatio - transitionStart) / transitionWidth

	smoothStep := position * position * (3 - 2*position)

	flowRate := freeFlow*(1-smoothStep) + submergedFlow*smoothStep

	regime := TransitionalFlow
	if position < 0.5 {
		regime = FreeFlow
	} else {
		regime = SubmergedFlow
	}

	return math.Max(0, flowRate), regime
}

func (h *HydroSimulator) calculateWeirFlowRate(waterLevelUp, waterLevelDown, headDiff float64) float64 {
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
	Cw := 1.84
	flowRate := sigma * Cw * h.gate.GateWidth * math.Pow(headDiff, 1.5)
	return math.Max(0, flowRate)
}

func (h *HydroSimulator) CalculateOrificeFlow(waterLevelUp, waterLevelDown, gateOpening float64) float64 {
	h.mu.RLock()
	defer h.mu.RUnlock()

	headDiff := waterLevelUp - waterLevelDown
	if headDiff <= 0 {
		return 0
	}

	openingHeight := gateOpening * h.gate.GateHeight
	if openingHeight <= 0 {
		return 0
	}

	relativeOpening := 0.0
	if waterLevelUp > 0 {
		relativeOpening = openingHeight / waterLevelUp
	}

	regime, submergenceRatio, _ := h.classifyFlowRegime(
		waterLevelUp, waterLevelDown, openingHeight, relativeOpening,
	)

	var flowRate float64

	switch regime {
	case FreeFlow:
		flowRate = h.calculateFreeFlowRate(waterLevelUp, openingHeight, relativeOpening)

	case SubmergedFlow:
		flowRate = h.calculateSubmergedFlowRate(
			waterLevelUp, waterLevelDown, openingHeight, relativeOpening, submergenceRatio,
		)

	case TransitionalFlow:
		flowRate, _ = h.calculateTransitionalFlowRate(
			waterLevelUp, waterLevelDown, openingHeight, relativeOpening, submergenceRatio,
		)

	case WeirFlow:
		flowRate = h.calculateWeirFlowRate(waterLevelUp, waterLevelDown, headDiff)

	default:
		flowRate = h.calculateFreeFlowRate(waterLevelUp, openingHeight, relativeOpening)
	}

	return math.Max(0, flowRate)
}

func (h *HydroSimulator) CalculateOrificeFlowDetailed(
	waterLevelUp, waterLevelDown, gateOpening float64,
) (float64, FlowRegime, float64, float64) {
	headDiff := waterLevelUp - waterLevelDown
	if headDiff <= 0 {
		return 0, FreeFlow, 0, 0
	}

	openingHeight := gateOpening * h.gate.GateHeight
	if openingHeight <= 0 {
		return 0, FreeFlow, 0, 0
	}

	relativeOpening := openingHeight / waterLevelUp
	regime, submergenceRatio, criticalDepth := h.classifyFlowRegime(
		waterLevelUp, waterLevelDown, openingHeight, relativeOpening,
	)

	var flowRate float64

	switch regime {
	case FreeFlow:
		flowRate = h.calculateFreeFlowRate(waterLevelUp, openingHeight, relativeOpening)

	case SubmergedFlow:
		flowRate = h.calculateSubmergedFlowRate(
			waterLevelUp, waterLevelDown, openingHeight, relativeOpening, submergenceRatio,
		)

	case TransitionalFlow:
		flowRate, regime = h.calculateTransitionalFlowRate(
			waterLevelUp, waterLevelDown, openingHeight, relativeOpening, submergenceRatio,
		)

	case WeirFlow:
		flowRate = h.calculateWeirFlowRate(waterLevelUp, waterLevelDown, headDiff)
	}

	return math.Max(0, flowRate), regime, submergenceRatio, criticalDepth
}

func (h *HydroSimulator) CalculateFillTime(
	targetLevelUp, initialChamberLevel, gateOpening float64,
) (float64, []models.WaterLevelPoint, []models.FlowRatePoint) {
	chamberArea := h.gate.ChamberLength * h.gate.ChamberWidth
	targetLevel := targetLevelUp

	if initialChamberLevel >= targetLevel {
		return 0,
			[]models.WaterLevelPoint{{Time: 0, WaterLevel: initialChamberLevel}},
			[]models.FlowRatePoint{{Time: 0, FlowRate: 0}}
	}

	dt := 0.25
	var totalTime float64
	var levelPoints []models.WaterLevelPoint
	var flowPoints []models.FlowRatePoint

	currentLevel := initialChamberLevel
	levelPoints = append(levelPoints, models.WaterLevelPoint{Time: 0, WaterLevel: currentLevel})

	maxIterations := 40000
	iterations := 0
	var accumulatedVolume float64

	chamberVolumeTarget := chamberArea * (targetLevel - initialChamberLevel)

	for currentLevel < targetLevel-0.0005 && iterations < maxIterations {
		flowRate := h.CalculateOrificeFlow(targetLevelUp, currentLevel, gateOpening)
		flowPoints = append(flowPoints, models.FlowRatePoint{Time: totalTime, FlowRate: flowRate})

		dtAdaptive := dt
		if flowRate > 0 {
			maxDH := (targetLevel - currentLevel) * 0.05
			theoreticalDT := maxDH * chamberArea / flowRate
			if theoreticalDT < dtAdaptive && theoreticalDT > 0.01 {
				dtAdaptive = theoreticalDT
			}
		}

		dv := flowRate * dtAdaptive
		dh := dv / chamberArea

		accumulatedVolume += dv
		currentLevel += dh
		totalTime += dtAdaptive
		iterations++

		levelThreshold := (targetLevel - initialChamberLevel) * 0.1
		if math.Mod(float64(iterations), 5) == 0 ||
			dh > levelThreshold ||
			currentLevel >= targetLevel {
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

	_ = accumulatedVolume
	_ = chamberVolumeTarget

	return totalTime, levelPoints, flowPoints
}

func (h *HydroSimulator) CalculateDrainTime(
	initialChamberLevel, targetLevelDown, gateOpening float64,
) (float64, []models.WaterLevelPoint, []models.FlowRatePoint) {
	chamberArea := h.gate.ChamberLength * h.gate.ChamberWidth
	targetLevel := targetLevelDown

	if initialChamberLevel <= targetLevel {
		return 0,
			[]models.WaterLevelPoint{{Time: 0, WaterLevel: initialChamberLevel}},
			[]models.FlowRatePoint{{Time: 0, FlowRate: 0}}
	}

	dt := 0.25
	var totalTime float64
	var levelPoints []models.WaterLevelPoint
	var flowPoints []models.FlowRatePoint

	currentLevel := initialChamberLevel
	levelPoints = append(levelPoints, models.WaterLevelPoint{Time: 0, WaterLevel: currentLevel})

	maxIterations := 40000
	iterations := 0

	for currentLevel > targetLevel+0.0005 && iterations < maxIterations {
		flowRate := h.CalculateOrificeFlow(currentLevel, targetLevelDown, gateOpening)
		flowPoints = append(flowPoints, models.FlowRatePoint{Time: totalTime, FlowRate: flowRate})

		dtAdaptive := dt
		if flowRate > 0 {
			maxDH := (currentLevel - targetLevel) * 0.05
			theoreticalDT := maxDH * chamberArea / flowRate
			if theoreticalDT < dtAdaptive && theoreticalDT > 0.01 {
				dtAdaptive = theoreticalDT
			}
		}

		dv := flowRate * dtAdaptive
		dh := dv / chamberArea

		currentLevel -= dh
		totalTime += dtAdaptive
		iterations++

		levelThreshold := (initialChamberLevel - targetLevel) * 0.1
		if math.Mod(float64(iterations), 5) == 0 ||
			dh > levelThreshold ||
			currentLevel <= targetLevel {
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

func (h *HydroSimulator) SimulateFullPassage(
	waterLevelUp, waterLevelDown, gateOpening float64, direction string,
) *models.SimulationResult {
	h.mu.Lock()
	defer h.mu.Unlock()

	var fillTime, drainTime float64
	var fillLevelCurve, drainLevelCurve []models.WaterLevelPoint
	var fillFlowCurve, drainFlowCurve []models.FlowRatePoint
	var initialLevel, targetLevel float64

	if direction == "upstream" {
		initialLevel = waterLevelDown
		targetLevel = waterLevelUp
		fillTime, fillLevelCurve, fillFlowCurve = h.CalculateFillTime(
			targetLevel, initialLevel, gateOpening,
		)
		drainTime = 0
	} else {
		initialLevel = waterLevelUp
		targetLevel = waterLevelDown
		drainTime, drainLevelCurve, drainFlowCurve = h.CalculateDrainTime(
			initialLevel, targetLevel, gateOpening,
		)
		fillTime = 0
	}

	var levelCurve []models.WaterLevelPoint
	var flowCurve []models.FlowRatePoint

	if direction == "upstream" {
		levelCurve = fillLevelCurve
		flowCurve = fillFlowCurve
	} else {
		levelCurve = drainLevelCurve
		flowCurve = drainFlowCurve
	}

	totalWaterVolume := math.Abs(initialLevel-targetLevel) *
		h.gate.ChamberLength * h.gate.ChamberWidth

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

	return &models.SimulationResult{
		FillTime:         fillTime,
		DrainTime:        drainTime,
		WaterLevelCurve:  levelCurve,
		FlowRateCurve:    flowCurve,
		MaxFlowRate:      maxFlowRate,
		AvgFlowRate:      avgFlowRate,
		TotalWaterVolume: totalWaterVolume,
	}
}

func (h *HydroSimulator) BatchSimulateOpenings(
	waterLevelUp, waterLevelDown float64, openings []float64, direction string,
	parallel bool,
) []*models.SimulationResult {
	results := make([]*models.SimulationResult, len(openings))

	if parallel && len(openings) > 3 {
		var wg sync.WaitGroup
		sem := make(chan struct{}, 8)

		for i, op := range openings {
			wg.Add(1)
			sem <- struct{}{}
			go func(idx int, opening float64) {
				defer wg.Done()
				defer func() { <-sem }()
				results[idx] = h.SimulateFullPassage(
					waterLevelUp, waterLevelDown, opening, direction,
				)
			}(i, op)
		}
		wg.Wait()
	} else {
		for i, op := range openings {
			results[i] = h.SimulateFullPassage(
				waterLevelUp, waterLevelDown, op, direction,
			)
		}
	}

	return results
}

func (h *HydroSimulator) CalculateOptimalOpening(
	targetFillTime, waterLevelUp, waterLevelDown float64, direction string,
) float64 {
	openings := make([]float64, 19)
	for i := range openings {
		openings[i] = 0.1 + float64(i)*0.05
	}

	results := h.BatchSimulateOpenings(waterLevelUp, waterLevelDown, openings, direction, true)

	bestOpening := 1.0
	minDiff := math.Inf(1)

	for i, result := range results {
		actualTime := result.FillTime
		if direction == "downstream" {
			actualTime = result.DrainTime
		}

		diff := math.Abs(actualTime - targetFillTime)
		if diff < minDiff {
			minDiff = diff
			bestOpening = openings[i]
		}
	}

	return bestOpening
}
