package config

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	DB           DBConfig
	MQTT         MQTTConfig
	Server       ServerConfig
	Hydro        HydroConfig
	GA           GAConfig
	HydraulicJSON HydraulicJSONConfig
	GAJSON       GAJSONConfig
}

type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
}

type MQTTConfig struct {
	Broker      string
	ClientID    string
	Username    string
	Password    string
	TopicAlert  string
}

type ServerConfig struct {
	Host string
	Port string
}

type HydroConfig struct {
	Gravity      float64
	WaterDensity float64
}

type GAConfig struct {
	PopulationSize int
	MaxGenerations int
	MutationRate   float64
	CrossoverRate  float64
}

type FlowRegimeParams struct {
	FreeFlowThreshold       float64 `json:"free_flow_threshold"`
	SubmergedThreshold  float64 `json:"submerged_flow_threshold"`
	WeirRelativeOpening float64 `json:"weir_relative_opening"`
	WeirHeadRatio      float64 `json:"weir_head_ratio"`
	FullySubmerged     float64 `json:"fully_submerged"`
	TransitionWidth    float64 `json:"transition_width"`
}

type ContractionCoeffParams struct {
	Base           float64 `json:"base"`
	LinearTerm     float64 `json:"linear_term"`
	QuadraticTerm  float64 `json:"quadratic_term"`
}

type SimulationParams struct {
	TimeStep       float64 `json:"time_step"`
	MaxIterations  int     `json:"max_iterations"`
	AdaptiveStepRatio float64 `json:"adaptive_step_ratio"`
	MinDT         float64 `json:"min_dt"`
}

type HydraulicJSONConfig struct {
	Gravity           float64                   `json:"gravity"`
	WaterDensity      float64                   `json:"water_density"`
	KinematicViscosity float64                   `json:"kinematic_viscosity"`
	DefaultCd         float64                   `json:"default_discharge_coefficient"`
	FlowRegime        FlowRegimeParams          `json:"flow_regime"`
	ContractionCoeff   ContractionCoeffParams   `json:"contraction_coefficient"`
	SubmergedCoeff    float64                   `json:"submerged_coefficient"`
	WeirCoeff         float64                   `json:"weir_coefficient"`
	Simulation        SimulationParams         `json:"simulation"`
}

type SelectionParams struct {
	Type                string `json:"type"`
	TournamentSizeSmall int    `json:"tournament_size_small"`
	TournamentSizeLarge int    `json:"tournament_size_large"`
	LargeThreshold      int    `json:"large_population_threshold"`
}

type EliteParams struct {
	Ratio    float64 `json:"ratio"`
	MinCount int     `json:"min_count"`
	MaxCount int     `json:"max_count"`
	PoolSize int     `json:"pool_size"`
}

type AdaptiveRatesParams struct {
	ConvergenceGapRatio    float64 `json:"convergence_gap_ratio"`
	HighConvCrossover  float64 `json:"high_convergence_crossover"`
	HighConvMutationMul float64 `json:"high_convergence_mutation_multiplier"`
	StagnantTrigger  int     `json:"stagnant_generations_trigger"`
	StagnantMutationMul float64 `json:"stagnant_mutation_multiplier"`
	StagnantCrossoverMul float64 `json:"stagnant_crossover_multiplier"`
	MaxMutationRate   float64 `json:"max_mutation_rate"`
	MaxCrossoverRate  float64 `json:"max_crossover_rate"`
}

type MutationParams struct {
	Types                 []string `json:"types"`
	LargePopExtraMutations int      `json:"large_population_extra_mutations"`
}

type LocalSearchParams struct {
	Enabled         bool `json:"enabled"`
	StagnantTrigger int  `json:"stagnant_trigger"`
	MaxIterations  int  `json:"max_iterations"`
}

type DiversityParams struct {
	BonusWeight float64 `json:"bonus_weight"`
	SampleSize    int     `json:"sample_size"`
}

type ParallelParams struct {
	MinWorkers     int `json:"min_workers"`
	MaxWorkers     int `json:"max_workers"`
	LargeShipThreshold int `json:"large_ship_threshold"`
}

type InitParams struct {
	HeuristicRatio      float64 `json:"heuristic_ratio"`
	LargePopMul          float64 `json:"large_population_multiplier"`
	LargeShipThreshold    int     `json:"large_ship_threshold"`
}

type StoppingParams struct {
	MaxStagnantSmall int `json:"max_stagnant_small"`
	MaxStagnantLarge int `json:"max_stagnant_large"`
	LargeShipThreshold int `json:"large_ship_threshold"`
}

type FitnessParams struct {
	ScaleFactor         float64 `json:"scale_factor"`
	PriorityWeightPower float64 `json:"priority_weight_power"`
	PriorityWeightBase  float64 `json:"priority_weight_base"`
}

type GAJSONConfig struct {
	PopulationSize int             `json:"population_size"`
	MaxGenerations int             `json:"max_generations"`
	MutationRate   float64           `json:"mutation_rate"`
	CrossoverRate  float64           `json:"crossover_rate"`
	Selection      SelectionParams      `json:"selection"`
	Elite          EliteParams          `json:"elite"`
	AdaptiveRates  AdaptiveRatesParams `json:"adaptive_rates"`
	Mutation       MutationParams     `json:"mutation"`
	LocalSearch    LocalSearchParams  `json:"local_search"`
	Diversity     DiversityParams     `json:"diversity"`
	Parallel       ParallelParams      `json:"parallel"`
	Initialization InitParams         `json:"initialization"`
	Stopping       StoppingParams      `json:"stopping"`
	Fitness        FitnessParams       `json:"fitness"`
}

var AppConfig Config

func Load() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}

	AppConfig = Config{
		DB: DBConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", "postgres"),
			Name:     getEnv("DB_NAME", "lingqu"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		MQTT: MQTTConfig{
			Broker:     getEnv("MQTT_BROKER", "tcp://localhost:1883"),
			ClientID:   getEnv("MQTT_CLIENT_ID", "dou_gate_server"),
			Username:   getEnv("MQTT_USERNAME", ""),
			Password:   getEnv("MQTT_PASSWORD", ""),
			TopicAlert: getEnv("MQTT_TOPIC_ALERT", "lingqu/alerts"),
		},
		Server: ServerConfig{
			Host: getEnv("SERVER_HOST", "0.0.0.0"),
			Port: getEnv("SERVER_PORT", "8080"),
		},
		Hydro: HydroConfig{
			Gravity:      getEnvFloat("GRAVITY", 9.81),
			WaterDensity: getEnvFloat("WATER_DENSITY", 1000.0),
		},
		GA: GAConfig{
			PopulationSize: getEnvInt("GA_POPULATION_SIZE", 100),
			MaxGenerations: getEnvInt("GA_MAX_GENERATIONS", 200),
			MutationRate:   getEnvFloat("GA_MUTATION_RATE", 0.1),
			CrossoverRate:  getEnvFloat("GA_CROSSOVER_RATE", 0.8),
		},
	}

	loadJSONConfigs()

	log.Println("Configuration loaded successfully")
}

func loadJSONConfigs() {
	configDir := getEnv("CONFIG_DIR", "config")

	hydroPath := filepath.Join(configDir, "hydraulic_params.json")
	if data, err := os.ReadFile(hydroPath); err == nil {
		var hydroCfg HydraulicJSONConfig
		if err := json.Unmarshal(data, &hydroCfg); err == nil {
			AppConfig.HydraulicJSON = hydroCfg
			log.Printf("Hydraulic params loaded from %s", hydroPath)
		}
	} else {
		log.Printf("Warning: Cannot load hydraulic params, using defaults. Error: %v", err)
		setDefaultHydraulicJSON()
	}

	gaPath := filepath.Join(configDir, "ga_params.json")
	if data, err := os.ReadFile(gaPath); err == nil {
		var gaCfg GAJSONConfig
		if err := json.Unmarshal(data, &gaCfg); err == nil {
			AppConfig.GAJSON = gaCfg
			log.Printf("GA params loaded from %s", gaPath)
		}
	} else {
		log.Printf("Warning: Cannot load GA params, using defaults. Error: %v", err)
		setDefaultGAJSON()
	}
}

func setDefaultHydraulicJSON() {
	AppConfig.HydraulicJSON = HydraulicJSONConfig{
		Gravity:           AppConfig.Hydro.Gravity,
		WaterDensity:      AppConfig.Hydro.WaterDensity,
		KinematicViscosity: 1e-6,
		DefaultCd:         0.63,
		FlowRegime: FlowRegimeParams{
			FreeFlowThreshold: 0.67,
			SubmergedThreshold: 0.88,
			WeirRelativeOpening: 0.75,
			WeirHeadRatio:      0.3,
			FullySubmerged:    0.97,
			TransitionWidth:   0.09,
		},
		ContractionCoeff: ContractionCoeffParams{
			Base:          0.615,
			LinearTerm:    0.105,
			QuadraticTerm: -0.02,
		},
		SubmergedCoeff: 0.92,
		WeirCoeff:      1.84,
		Simulation: SimulationParams{
			TimeStep:       0.25,
			MaxIterations:  40000,
			AdaptiveStepRatio: 0.05,
			MinDT:         0.01,
		},
	}
}

func setDefaultGAJSON() {
	AppConfig.GAJSON = GAJSONConfig{
		PopulationSize: AppConfig.GA.PopulationSize,
		MaxGenerations: AppConfig.GA.MaxGenerations,
		MutationRate:   AppConfig.GA.MutationRate,
		CrossoverRate:  AppConfig.GA.CrossoverRate,
		Selection: SelectionParams{
			Type:                "tournament",
			TournamentSizeSmall: 3,
			TournamentSizeLarge: 5,
			LargeThreshold:      30,
		},
		Elite: EliteParams{
			Ratio:    0.1,
			MinCount: 2,
			MaxCount: 10,
			PoolSize: 25,
		},
		AdaptiveRates: AdaptiveRatesParams{
			ConvergenceGapRatio:    0.1,
			HighConvCrossover:  0.9,
			HighConvMutationMul: 0.5,
			StagnantTrigger:  10,
			StagnantMutationMul: 2.0,
			StagnantCrossoverMul: 1.1,
			MaxMutationRate:   0.3,
			MaxCrossoverRate:  0.95,
		},
		Mutation: MutationParams{
			Types:                 []string{"swap", "inversion", "insertion"},
			LargePopExtraMutations: 2,
		},
		LocalSearch: LocalSearchParams{
			Enabled:         true,
			StagnantTrigger: 20,
			MaxIterations:  5,
		},
		Diversity: DiversityParams{
			BonusWeight: 1000,
			SampleSize:    10,
		},
		Parallel: ParallelParams{
			MinWorkers:     4,
			MaxWorkers:     8,
			LargeShipThreshold: 20,
		},
		Initialization: InitParams{
			HeuristicRatio:   0.2,
			LargePopMul:     1.5,
			LargeShipThreshold: 50,
		},
		Stopping: StoppingParams{
			MaxStagnantSmall: 50,
			MaxStagnantLarge: 80,
			LargeShipThreshold: 30,
		},
		Fitness: FitnessParams{
			ScaleFactor:        100000,
			PriorityWeightPower: 2.0,
			PriorityWeightBase:  25.0,
		},
	}
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func getEnvInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	v, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return v
}

func getEnvFloat(key string, defaultValue float64) float64 {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	v, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return defaultValue
	}
	return v
}
