package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	DB     DBConfig
	MQTT   MQTTConfig
	Server ServerConfig
	Hydro  HydroConfig
	GA     GAConfig
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

	log.Println("Configuration loaded successfully")
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
