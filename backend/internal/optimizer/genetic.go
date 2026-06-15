package optimizer

import (
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"

	"lingqu-dou-gate/internal/config"
	"lingqu-dou-gate/internal/models"
)

type Chromosome struct {
	Genes    []int
	Fitness  float64
	WaitTime float64
	Rank     int
	Crowding float64
}

type GAScheduler struct {
	gates         []models.DouGate
	ships         []models.ScheduleShip
	passageTime   float64
	population    []Chromosome
	elitePool     []Chromosome
	bestSolution  Chromosome
	bestHistory   []Chromosome
	config        config.GAConfig
	generation    int
	stagnantCount int
	numWorkers    int
	mu            sync.RWMutex
}

func NewGAScheduler(gates []models.DouGate, ships []models.ScheduleShip, passageTime float64) *GAScheduler {
	numWorkers := 4
	if len(ships) > 20 {
		numWorkers = 8
	}
	return &GAScheduler{
		gates:       gates,
		ships:       ships,
		passageTime: passageTime,
		config:      config.AppConfig.GA,
		numWorkers:  numWorkers,
	}
}

func (ga *GAScheduler) computeFitnessSingle(chromo *Chromosome) {
	totalWaitTime := 0.0
	weightedWaitTime := 0.0
	gateAvailable := make([]time.Time, len(ga.gates))

	now := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range gateAvailable {
		gateAvailable[i] = now
	}

	for _, geneIdx := range chromo.Genes {
		if geneIdx < 0 || geneIdx >= len(ga.ships) {
			continue
		}
		ship := ga.ships[geneIdx]
		bestGateIdx := 0
		bestStartTime := now
		minWait := math.Inf(1)

		shipArrival := ship.ArrivalTime
		if shipArrival.Before(now) {
			shipArrival = now
		}

		for gateIdx := range ga.gates {
			gateFree := gateAvailable[gateIdx]
			startTime := shipArrival
			if startTime.Before(gateFree) {
				startTime = gateFree
			}

			waitDuration := startTime.Sub(shipArrival).Seconds()
			if waitDuration < minWait {
				minWait = waitDuration
				bestGateIdx = gateIdx
				bestStartTime = startTime
			}
		}

		waitTime := bestStartTime.Sub(shipArrival).Seconds()
		totalWaitTime += waitTime
		priorityWeight := 1.0 + math.Pow(float64(ship.Priority), 2)/25.0
		weightedWaitTime += waitTime * priorityWeight

		endTime := bestStartTime.Add(time.Duration(ga.passageTime * float64(time.Second)))
		gateAvailable[bestGateIdx] = endTime
	}

	chromo.WaitTime = totalWaitTime

	baseFitness := 1.0 / (weightedWaitTime + 1.0) * 100000

	diversityBonus := ga.calculateDiversityBonus(*chromo) * 1000

	chromo.Fitness = baseFitness + diversityBonus
}

func (ga *GAScheduler) calculateDiversityBonus(chromo Chromosome) float64 {
	if len(ga.population) == 0 {
		return 0
	}

	totalDistance := 0.0
	sampleSize := minInt(10, len(ga.population))

	for i := 0; i < sampleSize; i++ {
		other := ga.population[rand.Intn(len(ga.population))]
		distance := 0.0
		for j := range chromo.Genes {
			if chromo.Genes[j] != other.Genes[j] {
				distance++
			}
		}
		totalDistance += distance / float64(len(chromo.Genes))
	}

	return totalDistance / float64(sampleSize)
}

func (ga *GAScheduler) evaluatePopulationParallel() {
	population := ga.population
	numJobs := len(population)
	numWorkers := ga.numWorkers
	if numJobs < numWorkers {
		numWorkers = numJobs
	}

	jobs := make(chan int, numJobs)
	var wg sync.WaitGroup

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				ga.computeFitnessSingle(&population[idx])
			}
		}()
	}

	for i := 0; i < numJobs; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
}

func (ga *GAScheduler) initializePopulation() {
	n := len(ga.ships)
	popSize := ga.config.PopulationSize
	if n > 50 {
		popSize = int(float64(popSize) * 1.5)
	}
	ga.population = make([]Chromosome, popSize)

	heuristicCount := int(float64(popSize) * 0.2)
	randomCount := popSize - heuristicCount

	for i := 0; i < heuristicCount; i++ {
		genes := ga.generateHeuristicChromosome(i)
		chromosome := Chromosome{Genes: genes}
		ga.population[i] = chromosome
	}

	for i := heuristicCount; i < popSize; i++ {
		genes := make([]int, n)
		for j := 0; j < n; j++ {
			genes[j] = j
		}
		rand.Shuffle(n, func(a, b int) {
			genes[a], genes[b] = genes[b], genes[a]
		})
		ga.population[i] = Chromosome{Genes: genes}
	}

	ga.evaluatePopulationParallel()
	ga.rankPopulation()

	ga.bestSolution = ga.population[0]
	for _, c := range ga.population {
		if c.Fitness > ga.bestSolution.Fitness {
			ga.bestSolution = c
		}
	}
	ga.elitePool = ga.getElite(int(float64(popSize) * 0.1))
}

func (ga *GAScheduler) generateHeuristicChromosome(seed int) []int {
	n := len(ga.ships)
	indices := make([]int, n)
	for i := 0; i < n; i++ {
		indices[i] = i
	}

	sort.Slice(indices, func(a, b int) bool {
		shipA := ga.ships[indices[a]]
		shipB := ga.ships[indices[b]]

		scoreA := -float64(shipA.Priority)*1000 + float64(shipA.ArrivalTime.Unix())
		scoreB := -float64(shipB.Priority)*1000 + float64(shipB.ArrivalTime.Unix())

		jitter := float64(seed%100) * 10
		return scoreA+jitter < scoreB
	})

	if seed%3 == 1 {
		for i := 0; i < n; i++ {
			if rand.Float64() < 0.2 {
				j := rand.Intn(n)
				indices[i], indices[j] = indices[j], indices[i]
			}
		}
	}

	return indices
}

func (ga *GAScheduler) rankPopulation() {
	sort.Slice(ga.population, func(i, j int) bool {
		return ga.population[i].Fitness > ga.population[j].Fitness
	})
	for i := range ga.population {
		ga.population[i].Rank = i
	}
}

func (ga *GAScheduler) getElite(count int) []Chromosome {
	if count > len(ga.population) {
		count = len(ga.population)
	}
	elite := make([]Chromosome, count)
	copy(elite, ga.population[:count])
	return elite
}

func (ga *GAScheduler) tournamentSelect(tournamentSize int) Chromosome {
	bestIdx := rand.Intn(len(ga.population))
	bestFitness := ga.population[bestIdx].Fitness

	for i := 1; i < tournamentSize; i++ {
		idx := rand.Intn(len(ga.population))
		if ga.population[idx].Fitness > bestFitness {
			bestFitness = ga.population[idx].Fitness
			bestIdx = idx
		}
	}

	return ga.population[bestIdx]
}

func (ga *GAScheduler) selectParent() Chromosome {
	tournamentSize := 3
	if len(ga.ships) > 30 {
		tournamentSize = 5
	}
	return ga.tournamentSelect(tournamentSize)
}

func (ga *GAScheduler) crossover(parent1, parent2 Chromosome) Chromosome {
	n := len(parent1.Genes)
	child := Chromosome{Genes: make([]int, n)}
	for i := range child.Genes {
		child.Genes[i] = -1
	}

	point1 := rand.Intn(n)
	point2 := rand.Intn(n)
	if point1 > point2 {
		point1, point2 = point2, point1
	}

	used := make(map[int]bool)
	for i := point1; i <= point2; i++ {
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

	for i := range child.Genes {
		if child.Genes[i] == -1 {
			for gene := 0; gene < n; gene++ {
				if !used[gene] {
					child.Genes[i] = gene
					used[gene] = true
					break
				}
			}
		}
	}

	return child
}

func (ga *GAScheduler) mutate(chromosome Chromosome, mutationRate float64) Chromosome {
	n := len(chromosome.Genes)
	child := Chromosome{Genes: make([]int, n)}
	copy(child.Genes, chromosome.Genes)

	numMutations := 1
	if n > 20 {
		numMutations = 2 + rand.Intn(3)
	}

	for m := 0; m < numMutations; m++ {
		if rand.Float64() < mutationRate {
			mutationType := rand.Intn(3)
			switch mutationType {
			case 0:
				i := rand.Intn(n)
				j := rand.Intn(n)
				child.Genes[i], child.Genes[j] = child.Genes[j], child.Genes[i]
			case 1:
				i := rand.Intn(n)
				j := rand.Intn(n)
				if i > j {
					i, j = j, i
				}
				for a, b := i, j; a < b; a, b = a+1, b-1 {
					child.Genes[a], child.Genes[b] = child.Genes[b], child.Genes[a]
				}
			case 2:
				i := rand.Intn(n)
				j := rand.Intn(n)
				if i > j {
					i, j = j, i
				}
				gene := child.Genes[j]
				for k := j; k > i; k-- {
					child.Genes[k] = child.Genes[k-1]
				}
				child.Genes[i] = gene
			}
		}
	}

	return child
}

func (ga *GAScheduler) localSearch(chromo Chromosome) Chromosome {
	best := chromo
	improved := true
	iterations := 0
	maxIter := 5

	for improved && iterations < maxIter {
		improved = false
		n := len(best.Genes)

		for i := 0; i < n-1 && !improved; i++ {
			neighbor := Chromosome{Genes: make([]int, n)}
			copy(neighbor.Genes, best.Genes)
			neighbor.Genes[i], neighbor.Genes[i+1] = neighbor.Genes[i+1], neighbor.Genes[i]

			ga.computeFitnessSingle(&neighbor)
			if neighbor.Fitness > best.Fitness {
				best = neighbor
				improved = true
			}
		}
		iterations++
	}

	return best
}

func (ga *GAScheduler) getAdaptiveRates() (crossoverRate, mutationRate float64) {
	avgFitness := 0.0
	for _, c := range ga.population {
		avgFitness += c.Fitness
	}
	avgFitness /= float64(len(ga.population))

	bestFitness := ga.bestSolution.Fitness

	if bestFitness-avgFitness > 0.1*bestFitness {
		crossoverRate = 0.9
		mutationRate = math.Min(0.05, ga.config.MutationRate*0.5)
	} else {
		crossoverRate = ga.config.CrossoverRate
		mutationRate = ga.config.MutationRate
	}

	if ga.stagnantCount > 10 {
		mutationRate = math.Min(0.3, mutationRate*2)
		crossoverRate = math.Min(0.95, crossoverRate*1.1)
	}

	return crossoverRate, mutationRate
}

func (ga *GAScheduler) deduplicatePopulation() {
	seen := make(map[string]bool)
	unique := make([]Chromosome, 0, len(ga.population))

	for _, c := range ga.population {
		key := ""
		for _, g := range c.Genes {
			key += "," + itoa(g)
		}
		if !seen[key] {
			seen[key] = true
			unique = append(unique, c)
		}
	}

	for len(unique) < len(ga.population) {
		n := len(ga.ships)
		genes := make([]int, n)
		for j := 0; j < n; j++ {
			genes[j] = j
		}
		rand.Shuffle(n, func(a, b int) {
			genes[a], genes[b] = genes[b], genes[a]
		})
		unique = append(unique, Chromosome{Genes: genes})
	}

	ga.population = unique
}

func (ga *GAScheduler) Optimize() (Chromosome, []Chromosome, int) {
	rand.Seed(time.Now().UnixNano())
	ga.initializePopulation()

	ga.bestHistory = []Chromosome{}
	ga.bestHistory = append(ga.bestHistory, ga.bestSolution)

	ga.generation = 0
	ga.stagnantCount = 0
	bestFitness := ga.bestSolution.Fitness

	eliteSize := int(float64(ga.config.PopulationSize) * 0.1)
	if eliteSize < 2 {
		eliteSize = 2
	}
	if eliteSize > 10 {
		eliteSize = 10
	}

	maxStagnant := 50
	if len(ga.ships) > 30 {
		maxStagnant = 80
	}

	for ga.generation < ga.config.MaxGenerations && ga.stagnantCount < maxStagnant {
		crossoverRate, mutationRate := ga.getAdaptiveRates()

		newPopulation := make([]Chromosome, 0, len(ga.population))

		currentElite := ga.getElite(eliteSize)
		newPopulation = append(newPopulation, currentElite...)

		if ga.stagnantCount > 20 {
			for i := range currentElite {
				improved := ga.localSearch(currentElite[i])
				newPopulation = append(newPopulation, improved)
			}
		}

		for len(newPopulation) < len(ga.population) {
			parent1 := ga.selectParent()
			parent2 := ga.selectParent()

			var child Chromosome
			if rand.Float64() < crossoverRate {
				child = ga.crossover(parent1, parent2)
			} else {
				child = Chromosome{Genes: make([]int, len(parent1.Genes))}
				copy(child.Genes, parent1.Genes)
			}

			child = ga.mutate(child, mutationRate)
			newPopulation = append(newPopulation, child)
		}

		for len(newPopulation) > len(ga.population) {
			newPopulation = newPopulation[:len(ga.population)]
		}

		ga.population = newPopulation
		ga.evaluatePopulationParallel()

		ga.deduplicatePopulation()

		ga.rankPopulation()

		ga.mu.Lock()
		prevBest := ga.bestSolution.Fitness
		ga.mu.Unlock()

		currentBest := ga.population[0]
		if currentBest.Fitness > prevBest {
			ga.mu.Lock()
			ga.bestSolution = currentBest
			ga.mu.Unlock()
		}

		ga.mu.RLock()
		if ga.bestSolution.Fitness > bestFitness {
			bestFitness = ga.bestSolution.Fitness
			ga.stagnantCount = 0

			ga.elitePool = append(ga.elitePool, ga.bestSolution)
			if len(ga.elitePool) > 50 {
				sort.Slice(ga.elitePool, func(i, j int) bool {
					return ga.elitePool[i].Fitness > ga.elitePool[j].Fitness
				})
				ga.elitePool = ga.elitePool[:25]
			}
		} else {
			ga.stagnantCount++
		}
		ga.mu.RUnlock()

		if ga.generation%5 == 0 {
			ga.mu.RLock()
			ga.bestHistory = append(ga.bestHistory, ga.bestSolution)
			ga.mu.RUnlock()
		}

		ga.generation++
	}

	ga.mu.Lock()
	ga.bestHistory = append(ga.bestHistory, ga.bestSolution)
	result := ga.bestSolution
	ga.mu.Unlock()

	return result, ga.bestHistory, ga.generation
}

func (ga *GAScheduler) GetScheduleItems(chromosome Chromosome) []models.ScheduleItem {
	ga.mu.RLock()
	gates := ga.gates
	ships := make([]models.ScheduleShip, len(ga.ships))
	copy(ships, ga.ships)
	passageTime := ga.passageTime
	ga.mu.RUnlock()

	items := make([]models.ScheduleItem, 0, len(ships))
	gateAvailable := make([]time.Time, len(gates))

	now := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range gateAvailable {
		gateAvailable[i] = now
	}

	for _, geneIdx := range chromosome.Genes {
		if geneIdx < 0 || geneIdx >= len(ships) {
			continue
		}
		ship := ships[geneIdx]
		bestGateIdx := 0
		bestStartTime := now
		minWait := math.Inf(1)

		shipArrival := ship.ArrivalTime
		if shipArrival.Before(now) {
			shipArrival = now
		}

		for gateIdx := range gates {
			gateFree := gateAvailable[gateIdx]
			startTime := shipArrival
			if startTime.Before(gateFree) {
				startTime = gateFree
			}

			waitDuration := startTime.Sub(shipArrival).Seconds()
			if waitDuration < minWait {
				minWait = waitDuration
				bestGateIdx = gateIdx
				bestStartTime = startTime
			}
		}

		waitTime := bestStartTime.Sub(shipArrival).Seconds()
		endTime := bestStartTime.Add(time.Duration(passageTime * float64(time.Second)))

		items = append(items, models.ScheduleItem{
			ShipID:    ship.ShipID,
			ShipName:  ship.ShipName,
			StartTime: bestStartTime,
			EndTime:   endTime,
			WaitTime:  waitTime,
			Priority:  ship.Priority,
			Direction: ship.Direction,
		})

		gateAvailable[bestGateIdx] = endTime
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].StartTime.Before(items[j].StartTime)
	})

	return items
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	digits := []rune{}
	for n > 0 {
		digits = append([]rune{rune('0' + n%10)}, digits...)
		n /= 10
	}
	if negative {
		digits = append([]rune{'-'}, digits...)
	}
	return string(digits)
}
