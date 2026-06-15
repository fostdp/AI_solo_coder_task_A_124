package scheduler_ga

import (
	"log"
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

type OptimizeRequest struct {
	Gates       []models.DouGate
	Ships       []models.ScheduleShip
	PassageTime float64
	ReplyChan   chan *OptimizeResult
}

type OptimizeResult struct {
	Schedule      []models.ScheduleItem `json:"schedule"`
	TotalWaitTime float64               `json:"total_wait_time"`
	Fitness       float64               `json:"fitness"`
	Generations   int                   `json:"generations"`
	HistoryCount  int                   `json:"history_count"`
	Error         error                 `json:"-"`
}

type GAScheduler struct {
	mu           sync.RWMutex
	running      bool
	requestChan  chan OptimizeRequest
	stopChan     chan struct{}
	wg           sync.WaitGroup
	params       config.GAJSONConfig
	workerCount  int

	elitePool   []Chromosome
	bestSolution Chromosome
}

func NewGAScheduler(workerCount int) *GAScheduler {
	if workerCount <= 0 {
		workerCount = 1
	}
	return &GAScheduler{
		requestChan: make(chan OptimizeRequest, 20),
		stopChan:    make(chan struct{}),
		params:      config.AppConfig.GAJSON,
		workerCount: workerCount,
	}
}

func (ga *GAScheduler) RequestChannel() chan<- OptimizeRequest {
	return ga.requestChan
}

func (ga *GAScheduler) Submit(req OptimizeRequest) {
	select {
	case ga.requestChan <- req:
	default:
		if req.ReplyChan != nil {
			req.ReplyChan <- &OptimizeResult{
				Error: &SchedulerError{Message: "scheduler queue full"},
			}
		}
	}
}

type SchedulerError struct {
	Message string
}

func (e *SchedulerError) Error() string {
	return e.Message
}

func (ga *GAScheduler) Start() {
	ga.mu.Lock()
	defer ga.mu.Unlock()

	if ga.running {
		return
	}
	ga.running = true
	ga.params = config.AppConfig.GAJSON

	for i := 0; i < ga.workerCount; i++ {
		ga.wg.Add(1)
		go ga.worker(i)
	}

	log.Printf("GA scheduler started with %d workers", ga.workerCount)
}

func (ga *GAScheduler) Stop() {
	ga.mu.Lock()
	defer ga.mu.Unlock()

	if !ga.running {
		return
	}
	ga.running = false

	close(ga.stopChan)
	ga.wg.Wait()
	close(ga.requestChan)

	log.Println("GA scheduler stopped")
}

func (ga *GAScheduler) worker(id int) {
	defer ga.wg.Done()

	for {
		select {
		case <-ga.stopChan:
			return
		case req, ok := <-ga.requestChan:
			if !ok {
				return
			}
			result := ga.optimize(req)
			if req.ReplyChan != nil {
				select {
				case req.ReplyChan <- result:
				default:
				}
			}
		}
	}
}

func (ga *GAScheduler) optimize(req OptimizeRequest) *OptimizeResult {
	ships := req.Ships
	gates := req.Gates
	passageTime := req.PassageTime

	if len(ships) == 0 {
		return &OptimizeResult{
			Schedule:      []models.ScheduleItem{},
			TotalWaitTime: 0,
			Fitness:       0,
			Generations:   0,
		}
	}

	if len(gates) == 0 {
		gates = []models.DouGate{{ID: 1, GateWidth: 6.0}}
	}

	params := ga.params
	n := len(ships)
	popSize := params.PopulationSize
	if n > params.Initialization.LargeShipThreshold {
		popSize = int(float64(popSize) * params.Initialization.LargePopMul)
	}

	population := ga.initializePopulation(ships, gates, passageTime, popSize, params)
	bestSolution := ga.getBest(population)
	bestHistory := []Chromosome{bestSolution}

	generation := 0
	stagnantCount := 0
	bestFitness := bestSolution.Fitness

	eliteSize := int(float64(popSize) * params.Elite.Ratio)
	if eliteSize < params.Elite.MinCount {
		eliteSize = params.Elite.MinCount
	}
	if eliteSize > params.Elite.MaxCount {
		eliteSize = params.Elite.MaxCount
	}

	maxStagnant := params.Stopping.MaxStagnantSmall
	if len(ships) > params.Stopping.LargeShipThreshold {
		maxStagnant = params.Stopping.MaxStagnantLarge
	}

	for generation < params.MaxGenerations && stagnantCount < maxStagnant {
		crossoverRate, mutationRate := ga.getAdaptiveRates(population, bestSolution, stagnantCount, params)

		newPopulation := make([]Chromosome, 0, popSize)
		currentElite := ga.getTopElite(population, eliteSize)
		newPopulation = append(newPopulation, currentElite...)

		if params.LocalSearch.Enabled && stagnantCount > params.LocalSearch.StagnantTrigger {
			for i := range currentElite {
				improved := ga.localSearch(currentElite[i], ships, gates, passageTime, params)
				newPopulation = append(newPopulation, improved)
			}
		}

		for len(newPopulation) < popSize {
			parent1 := ga.tournamentSelect(population, params)
			parent2 := ga.tournamentSelect(population, params)

			var child Chromosome
			if rand.Float64() < crossoverRate {
				child = ga.crossover(parent1, parent2)
			} else {
				child = Chromosome{Genes: make([]int, len(parent1.Genes))}
				copy(child.Genes, parent1.Genes)
			}

			child = ga.mutate(child, mutationRate, params)
			newPopulation = append(newPopulation, child)
		}

		for len(newPopulation) > popSize {
			newPopulation = newPopulation[:popSize]
		}

		population = newPopulation
		ga.evaluatePopulationParallel(population, ships, gates, passageTime, params)
		ga.deduplicatePopulation(&population)
		ga.rankPopulation(&population)

		currentBest := population[0]
		if currentBest.Fitness > bestFitness {
			bestSolution = currentBest
			bestFitness = currentBest.Fitness
			stagnantCount = 0
		} else {
			stagnantCount++
		}

		if generation%5 == 0 {
			bestHistory = append(bestHistory, bestSolution)
		}

		generation++
	}

	bestHistory = append(bestHistory, bestSolution)
	scheduleItems := ga.buildSchedule(bestSolution, ships, gates, passageTime)

	return &OptimizeResult{
		Schedule:      scheduleItems,
		TotalWaitTime: bestSolution.WaitTime,
		Fitness:       bestSolution.Fitness,
		Generations:   generation,
		HistoryCount:  len(bestHistory),
	}
}

func (ga *GAScheduler) initializePopulation(
	ships []models.ScheduleShip,
	gates []models.DouGate,
	passageTime float64,
	popSize int,
	params config.GAJSONConfig,
) []Chromosome {
	n := len(ships)
	population := make([]Chromosome, popSize)

	heuristicCount := int(float64(popSize) * params.Initialization.HeuristicRatio)
	randomCount := popSize - heuristicCount

	for i := 0; i < heuristicCount; i++ {
		genes := ga.generateHeuristicChromosome(ships, i)
		chromosome := Chromosome{Genes: genes}
		population[i] = chromosome
	}

	for i := heuristicCount; i < popSize; i++ {
		genes := make([]int, n)
		for j := 0; j < n; j++ {
			genes[j] = j
		}
		rand.Shuffle(n, func(a, b int) {
			genes[a], genes[b] = genes[b], genes[a]
		})
		population[i] = Chromosome{Genes: genes}
	}

	ga.evaluatePopulationParallel(population, ships, gates, passageTime, params)
	ga.rankPopulation(&population)

	return population
}

func (ga *GAScheduler) generateHeuristicChromosome(ships []models.ScheduleShip, seed int) []int {
	n := len(ships)
	indices := make([]int, n)
	for i := 0; i < n; i++ {
		indices[i] = i
	}

	sort.Slice(indices, func(a, b int) bool {
		shipA := ships[indices[a]]
		shipB := ships[indices[b]]

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

func (ga *GAScheduler) evaluatePopulationParallel(
	population []Chromosome,
	ships []models.ScheduleShip,
	gates []models.DouGate,
	passageTime float64,
	params config.GAJSONConfig,
) {
	numJobs := len(population)
	numWorkers := params.Parallel.MinWorkers
	if len(ships) > params.Parallel.LargeShipThreshold {
		numWorkers = params.Parallel.MaxWorkers
	}
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
				ga.computeFitness(&population[idx], ships, gates, passageTime, params)
			}
		}()
	}

	for i := 0; i < numJobs; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
}

func (ga *GAScheduler) computeFitness(
	chromo *Chromosome,
	ships []models.ScheduleShip,
	gates []models.DouGate,
	passageTime float64,
	params config.GAJSONConfig,
) {
	totalWaitTime := 0.0
	weightedWaitTime := 0.0
	gateAvailable := make([]time.Time, len(gates))

	now := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range gateAvailable {
		gateAvailable[i] = now
	}

	for _, geneIdx := range chromo.Genes {
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
		totalWaitTime += waitTime
		priorityWeight := 1.0 + math.Pow(float64(ship.Priority), params.Fitness.PriorityWeightPower)/params.Fitness.PriorityWeightBase
		weightedWaitTime += waitTime * priorityWeight

		endTime := bestStartTime.Add(time.Duration(passageTime * float64(time.Second)))
		gateAvailable[bestGateIdx] = endTime
	}

	chromo.WaitTime = totalWaitTime
	baseFitness := 1.0 / (weightedWaitTime + 1.0) * params.Fitness.ScaleFactor
	chromo.Fitness = baseFitness
}

func (ga *GAScheduler) rankPopulation(population *[]Chromosome) {
	sort.Slice(*population, func(i, j int) bool {
		return (*population)[i].Fitness > (*population)[j].Fitness
	})
	for i := range *population {
		(*population)[i].Rank = i
	}
}

func (ga *GAScheduler) getBest(population []Chromosome) Chromosome {
	best := population[0]
	for _, c := range population {
		if c.Fitness > best.Fitness {
			best = c
		}
	}
	return best
}

func (ga *GAScheduler) getTopElite(population []Chromosome, count int) []Chromosome {
	if count > len(population) {
		count = len(population)
	}
	elite := make([]Chromosome, count)
	copy(elite, population[:count])
	return elite
}

func (ga *GAScheduler) tournamentSelect(population []Chromosome, params config.GAJSONConfig) Chromosome {
	size := params.Selection.TournamentSizeSmall
	if len(population) > params.Selection.LargeThreshold {
		size = params.Selection.TournamentSizeLarge
	}

	bestIdx := rand.Intn(len(population))
	bestFitness := population[bestIdx].Fitness

	for i := 1; i < size; i++ {
		idx := rand.Intn(len(population))
		if population[idx].Fitness > bestFitness {
			bestFitness = population[idx].Fitness
			bestIdx = idx
		}
	}

	return population[bestIdx]
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

func (ga *GAScheduler) mutate(chromosome Chromosome, mutationRate float64, params config.GAJSONConfig) Chromosome {
	n := len(chromosome.Genes)
	child := Chromosome{Genes: make([]int, n)}
	copy(child.Genes, chromosome.Genes)

	numMutations := 1
	if n > 20 {
		numMutations = 1 + params.Mutation.LargePopExtraMutations
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

func (ga *GAScheduler) getAdaptiveRates(
	population []Chromosome,
	bestSolution Chromosome,
	stagnantCount int,
	params config.GAJSONConfig,
) (crossoverRate, mutationRate float64) {
	avgFitness := 0.0
	for _, c := range population {
		avgFitness += c.Fitness
	}
	avgFitness /= float64(len(population))

	bestFitness := bestSolution.Fitness
	adaptiveCfg := params.AdaptiveRates

	if bestFitness-avgFitness > adaptiveCfg.ConvergenceGapRatio*bestFitness {
		crossoverRate = adaptiveCfg.HighConvCrossover
		mutationRate = params.MutationRate * adaptiveCfg.HighConvMutationMul
	} else {
		crossoverRate = params.CrossoverRate
		mutationRate = params.MutationRate
	}

	if stagnantCount > adaptiveCfg.StagnantTrigger {
		mutationRate = math.Min(adaptiveCfg.MaxMutationRate, mutationRate*adaptiveCfg.StagnantMutationMul)
		crossoverRate = math.Min(adaptiveCfg.MaxCrossoverRate, crossoverRate*adaptiveCfg.StagnantCrossoverMul)
	}

	return crossoverRate, mutationRate
}

func (ga *GAScheduler) localSearch(
	chromo Chromosome,
	ships []models.ScheduleShip,
	gates []models.DouGate,
	passageTime float64,
	params config.GAJSONConfig,
) Chromosome {
	best := chromo
	improved := true
	iterations := 0
	maxIter := params.LocalSearch.MaxIterations

	for improved && iterations < maxIter {
		improved = false
		n := len(best.Genes)

		for i := 0; i < n-1 && !improved; i++ {
			neighbor := Chromosome{Genes: make([]int, n)}
			copy(neighbor.Genes, best.Genes)
			neighbor.Genes[i], neighbor.Genes[i+1] = neighbor.Genes[i+1], neighbor.Genes[i]

			ga.computeFitness(&neighbor, ships, gates, passageTime, params)
			if neighbor.Fitness > best.Fitness {
				best = neighbor
				improved = true
			}
		}
		iterations++
	}

	return best
}

func (ga *GAScheduler) deduplicatePopulation(population *[]Chromosome) {
	seen := make(map[string]bool)
	unique := make([]Chromosome, 0, len(*population))

	for _, c := range *population {
		key := ""
		for _, g := range c.Genes {
			key += "," + itoa(g)
		}
		if !seen[key] {
			seen[key] = true
			unique = append(unique, c)
		}
	}

	for len(unique) < len(*population) {
		n := len((*population)[0].Genes)
		genes := make([]int, n)
		for j := 0; j < n; j++ {
			genes[j] = j
		}
		rand.Shuffle(n, func(a, b int) {
			genes[a], genes[b] = genes[b], genes[a]
		})
		unique = append(unique, Chromosome{Genes: genes})
	}

	*population = unique
}

func (ga *GAScheduler) buildSchedule(
	chromosome Chromosome,
	ships []models.ScheduleShip,
	gates []models.DouGate,
	passageTime float64,
) []models.ScheduleItem {
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

func (ga *GAScheduler) ReloadConfig() {
	ga.mu.Lock()
	defer ga.mu.Unlock()
	ga.params = config.AppConfig.GAJSON
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

func init() {
	rand.Seed(time.Now().UnixNano())
}
