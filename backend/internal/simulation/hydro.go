package simulation

import (
	"math"
	"lingqu-dou-gate/internal/config"
	"lingqu-dou-gate/internal/models"
)

type HydroSimulator struct {
	gravity    float64
	gate       models.DouGate
}

func NewHydroSimulator(gate models.DouGate) *HydroSimulator {
	return &HydroSimulator{
		gravity: config.AppConfig.Hydro.Gravity,
		gate:    gate,
	}
}

func (h *HydroSimulator) CalculateOrificeFlow(waterLevelUp, waterLevelDown, gateOpening float64) float64 {
	headDiff := waterLevelUp - waterLevelDown
	if headDiff <= 0 {
		return 0
	}

	openingHeight := gateOpening * h.gate.GateHeight
	if openingHeight <= 0 {
		return 0
	}

	var flowRate float64
	submergedThreshold := waterLevelDown / (waterLevelUp - 0.1)

	if waterLevelDown < waterLevelUp-openingHeight {
		flowRate = h.gate.DischargeCoefficient * openingHeight * h.gate.GateWidth *
			math.Sqrt(2*h.gravity*(waterLevelUp-openingHeight))
	} else if submergedThreshold > 0.8 {
		flowRate = h.gate.DischargeCoefficient * openingHeight * h.gate.GateWidth *
			math.Sqrt(2 * h.gravity * headDiff)
	} else {
		flowRate = h.gate.DischargeCoefficient * openingHeight * h.gate.GateWidth *
			math.Sqrt(2*h.gravity*headDiff) * 0.85
	}

	if flowRate < 0 {
		flowRate = 0
	}

	return flowRate
}

func (h *HydroSimulator) CalculateFillTime(targetLevelUp, initialChamberLevel, gateOpening float64) (float64, []models.WaterLevelPoint, []models.FlowRatePoint) {
	chamberArea := h.gate.ChamberLength * h.gate.ChamberWidth
	targetLevel := targetLevelUp

	if initialChamberLevel >= targetLevel {
		return 0, []models.WaterLevelPoint{{Time: 0, WaterLevel: initialChamberLevel}},
			[]models.FlowRatePoint{{Time: 0, FlowRate: 0}}
	}

	dt := 0.5
	var totalTime float64
	var levelPoints []models.WaterLevelPoint
	var flowPoints []models.FlowRatePoint

	currentLevel := initialChamberLevel
	levelPoints = append(levelPoints, models.WaterLevelPoint{Time: 0, WaterLevel: currentLevel})

	maxIterations := 10000
	iterations := 0

	for currentLevel < targetLevel-0.001 && iterations < maxIterations {
		flowRate := h.CalculateOrificeFlow(targetLevelUp, currentLevel, gateOpening)
		flowPoints = append(flowPoints, models.FlowRatePoint{Time: totalTime, FlowRate: flowRate})

		dv := flowRate * dt
		dh := dv / chamberArea

		currentLevel += dh
		totalTime += dt
		iterations++

		if math.Mod(float64(iterations), 10) == 0 || currentLevel >= targetLevel {
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

func (h *HydroSimulator) CalculateDrainTime(initialChamberLevel, targetLevelDown, gateOpening float64) (float64, []models.WaterLevelPoint, []models.FlowRatePoint) {
	chamberArea := h.gate.ChamberLength * h.gate.ChamberWidth
	targetLevel := targetLevelDown

	if initialChamberLevel <= targetLevel {
		return 0, []models.WaterLevelPoint{{Time: 0, WaterLevel: initialChamberLevel}},
			[]models.FlowRatePoint{{Time: 0, FlowRate: 0}}
	}

	dt := 0.5
	var totalTime float64
	var levelPoints []models.WaterLevelPoint
	var flowPoints []models.FlowRatePoint

	currentLevel := initialChamberLevel
	levelPoints = append(levelPoints, models.WaterLevelPoint{Time: 0, WaterLevel: currentLevel})

	maxIterations := 10000
	iterations := 0

	for currentLevel > targetLevel+0.001 && iterations < maxIterations {
		flowRate := h.CalculateOrificeFlow(currentLevel, targetLevelDown, gateOpening)
		flowPoints = append(flowPoints, models.FlowRatePoint{Time: totalTime, FlowRate: flowRate})

		dv := flowRate * dt
		dh := dv / chamberArea

		currentLevel -= dh
		totalTime += dt
		iterations++

		if math.Mod(float64(iterations), 10) == 0 || currentLevel <= targetLevel {
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

func (h *HydroSimulator) SimulateFullPassage(waterLevelUp, waterLevelDown, gateOpening float64, direction string) *models.SimulationResult {
	var fillTime, drainTime float64
	var fillLevelCurve, drainLevelCurve []models.WaterLevelPoint
	var fillFlowCurve, drainFlowCurve []models.FlowRatePoint
	var initialLevel, targetLevel float64

	if direction == "upstream" {
		initialLevel = waterLevelDown
		targetLevel = waterLevelUp
		fillTime, fillLevelCurve, fillFlowCurve = h.CalculateFillTime(targetLevel, initialLevel, gateOpening)
		drainTime = 0
	} else {
		initialLevel = waterLevelUp
		targetLevel = waterLevelDown
		drainTime, drainLevelCurve, drainFlowCurve = h.CalculateDrainTime(initialLevel, targetLevel, gateOpening)
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

	totalWaterVolume := math.Abs(initialLevel-targetLevel) * h.gate.ChamberLength * h.gate.ChamberWidth

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

func (h *HydroSimulator) CalculateOptimalOpening(targetFillTime, waterLevelUp, waterLevelDown float64, direction string) float64 {
	bestOpening := 1.0
	minDiff := math.Inf(1)

	for opening := 0.1; opening <= 1.0; opening += 0.05 {
		result := h.SimulateFullPassage(waterLevelUp, waterLevelDown, opening, direction)
		actualTime := result.FillTime
		if direction == "downstream" {
			actualTime = result.DrainTime
		}

		diff := math.Abs(actualTime - targetFillTime)
		if diff < minDiff {
			minDiff = diff
			bestOpening = opening
		}
	}

	return bestOpening
}
