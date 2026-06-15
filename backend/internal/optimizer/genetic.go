package optimizer

import (
	"math"
	"math/rand"
	"sort"
	"time"

	"lingqu-dou-gate/internal/config"
	"lingqu-dou-gate/internal/models"
)

type Chromosome struct {
	Genes    []int
	Fitness  float64
	WaitTime float64
}

type GAScheduler struct {
	gates         []models.DouGate
	ships         []models.ScheduleShip
	passageTime   float64
	population    []Chromosome
	bestSolution  Chromosome
	config        config.GAConfig
}

func NewGAScheduler(gates []models.DouGate, ships []models.ScheduleShip, passageTime float64) *GAScheduler {
	return &GAScheduler{
		gates:       gates,
		ships:       ships,
		passageTime: passageTime,
		config:      config.AppConfig.GA,
	}
}

func (ga *GAScheduler) initializePopulation() {
	ga.population = make([]Chromosome, ga.config.PopulationSize)
	n := len(ga.ships)

	for i := 0; i < ga.config.PopulationSize; i++ {
		genes := make([]int, n)
		for j := 0; j < n; j++ {
			genes[j] = j
		}
		rand.Shuffle(n, func(i, j int) {
			genes[i], genes[j] = genes[j], genes[i]
		})

		chromosome := Chromosome{Genes: genes}
		ga.calculateFitness(&chromosome)
		ga.population[i] = chromosome
	}

	ga.bestSolution = ga.population[0]
	for _, c := range ga.population {
		if c.Fitness > ga.bestSolution.Fitness {
			ga.bestSolution = c
		}
	}
}

func (ga *GAScheduler) calculateFitness(chromosome *Chromosome) {
	totalWaitTime := 0.0
	weightedWaitTime := 0.0
	gateAvailable := make([]time.Time, len(ga.gates))

	for i := range gateAvailable {
		gateAvailable[i] = time.Time{}
	}

	for _, geneIdx := range chromosome.Genes {
		ship := ga.ships[geneIdx]
		bestGateIdx := 0
		bestStartTime := time.Time{}
		minWait := math.Inf(1)

		for gateIdx, gate := range ga.gates {
			gateFree := gateAvailable[gateIdx]
			startTime := ship.ArrivalTime
			if startTime.Before(gateFree) {
				startTime = gateFree
			}

			waitDuration := startTime.Sub(ship.ArrivalTime).Seconds()
			if waitDuration < minWait {
				minWait = waitDuration
				bestGateIdx = gateIdx
				bestStartTime = startTime
			}
		}

		waitTime := bestStartTime.Sub(ship.ArrivalTime).Seconds()
		totalWaitTime += waitTime
		weightedWaitTime += waitTime * float64(ship.Priority)

		endTime := bestStartTime.Add(time.Duration(ga.passageTime * float64(time.Second)))
		gateAvailable[bestGateIdx] = endTime
	}

	chromosome.WaitTime = totalWaitTime
	chromosome.Fitness = 1.0 / (weightedWaitTime + 1.0) * 10000
}

func (ga *GAScheduler) selectParent() Chromosome {
	totalFitness := 0.0
	for _, c := range ga.population {
		totalFitness += c.Fitness
	}

	r := rand.Float64() * totalFitness
	cumulative := 0.0
	for _, c := range ga.population {
		cumulative += c.Fitness
		if cumulative >= r {
			return c
		}
	}

	return ga.population[0]
}

func (ga *GAScheduler) crossover(parent1, parent2 Chromosome) Chromosome {
	n := len(parent1.Genes)
	child := Chromosome{Genes: make([]int, n)}
	for i := range child.Genes {
		child.Genes[i] = -1
	}

	start := rand.Intn(n)
	end := rand.Intn(n)
	if start > end {
		start, end = end, start
	}

	used := make(map[int]bool)
	for i := start; i <= end; i++ {
		child.Genes[i] = parent1.Genes[i]
		used[parent1.Genes[i]] = true
	}

	j := 0
	for i := 0; i < n; i++ {
		if child.Genes[i] == -1 {
			for j < n && used[parent2.Genes[j]] {
				j++
			}
			if j < n {
				child.Genes[i] = parent2.Genes[j]
				used[parent2.Genes[j]] = true
				j++
			}
		}
	}

	ga.calculateFitness(&child)
	return child
}

func (ga *GAScheduler) mutate(chromosome Chromosome) Chromosome {
	n := len(chromosome.Genes)
	child := Chromosome{Genes: make([]int, n)}
	copy(child.Genes, chromosome.Genes)

	i := rand.Intn(n)
	j := rand.Intn(n)
	child.Genes[i], child.Genes[j] = child.Genes[j], child.Genes[i]

	ga.calculateFitness(&child)
	return child
}

func (ga *GAScheduler) Optimize() (Chromosome, []Chromosome, int) {
	rand.Seed(time.Now().UnixNano())
	ga.initializePopulation()

	var bestHistory []Chromosome
	bestHistory = append(bestHistory, ga.bestSolution)

	generation := 0
	stagnant := 0
	bestFitness := ga.bestSolution.Fitness

	for generation < ga.config.MaxGenerations && stagnant < 50 {
		newPopulation := make([]Chromosome, 0, ga.config.PopulationSize)

		newPopulation = append(newPopulation, ga.bestSolution)
		newPopulation = append(newPopulation, ga.bestSolution)

		for len(newPopulation) < ga.config.PopulationSize {
			parent1 := ga.selectParent()
			parent2 := ga.selectParent()

			var child Chromosome
			if rand.Float64() < ga.config.CrossoverRate {
				child = ga.crossover(parent1, parent2)
			} else {
				child = parent1
			}

			if rand.Float64() < ga.config.MutationRate {
				child = ga.mutate(child)
			}

			newPopulation = append(newPopulation, child)
		}

		ga.population = newPopulation

		for _, c := range ga.population {
			if c.Fitness > ga.bestSolution.Fitness {
				ga.bestSolution = c
			}
		}

		if ga.bestSolution.Fitness > bestFitness {
			bestFitness = ga.bestSolution.Fitness
			stagnant = 0
		} else {
			stagnant++
		}

		if generation%10 == 0 {
			bestHistory = append(bestHistory, ga.bestSolution)
		}

		generation++
	}

	bestHistory = append(bestHistory, ga.bestSolution)

	return ga.bestSolution, bestHistory, generation
}

func (ga *GAScheduler) GetScheduleItems(chromosome Chromosome) []models.ScheduleItem {
	items := make([]models.ScheduleItem, 0, len(ga.ships))
	gateAvailable := make([]time.Time, len(ga.gates))
	gateShips := make([][]int, len(ga.gates))

	for i := range gateAvailable {
		gateAvailable[i] = time.Time{}
	}

	for _, geneIdx := range chromosome.Genes {
		ship := ga.ships[geneIdx]
		bestGateIdx := 0
		bestStartTime := time.Time{}
		minWait := math.Inf(1)

		for gateIdx := range ga.gates {
			gateFree := gateAvailable[gateIdx]
			startTime := ship.ArrivalTime
			if startTime.Before(gateFree) {
				startTime = gateFree
			}

			waitDuration := startTime.Sub(ship.ArrivalTime).Seconds()
			if waitDuration < minWait {
				minWait = waitDuration
				bestGateIdx = gateIdx
				bestStartTime = startTime
			}
		}

		waitTime := bestStartTime.Sub(ship.ArrivalTime).Seconds()
		endTime := bestStartTime.Add(time.Duration(ga.passageTime * float64(time.Second)))

		items = append(items, models.ScheduleItem{
			ShipID:     ship.ShipID,
			ShipName:   ship.ShipName,
			StartTime:  bestStartTime,
			EndTime:    endTime,
			WaitTime:   waitTime,
			Priority:   ship.Priority,
			Direction:  ship.Direction,
		})

		gateAvailable[bestGateIdx] = endTime
		gateShips[bestGateIdx] = append(gateShips[bestGateIdx], geneIdx)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].StartTime.Before(items[j].StartTime)
	})

	return items
}
