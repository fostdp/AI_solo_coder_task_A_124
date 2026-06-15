package services

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"lingqu-dou-gate/internal/config"
	"lingqu-dou-gate/internal/models"
)

var MQTTClient mqtt.Client

func InitMQTT() {
	cfg := config.AppConfig.MQTT

	opts := mqtt.NewClientOptions().AddBroker(cfg.Broker)
	opts.SetClientID(cfg.ClientID)
	if cfg.Username != "" {
		opts.SetUsername(cfg.Username)
		opts.SetPassword(cfg.Password)
	}

	opts.OnConnect = func(client mqtt.Client) {
		log.Println("MQTT client connected successfully")
	}

	opts.OnConnectionLost = func(client mqtt.Client, err error) {
		log.Printf("MQTT connection lost: %v", err)
	}

	opts.AutoReconnect = true
	opts.KeepAlive = 30 * time.Second

	MQTTClient = mqtt.NewClient(opts)

	go func() {
		token := MQTTClient.Connect()
		if token.Wait() && token.Error() != nil {
			log.Printf("Warning: Failed to connect to MQTT broker: %v", token.Error())
			log.Println("Continuing with limited MQTT functionality...")
		}
	}()
}

func PublishAlert(alert models.Alert) error {
	if MQTTClient == nil || !MQTTClient.IsConnected() {
		log.Println("Warning: MQTT not connected, skipping alert publish")
		return fmt.Errorf("MQTT not connected")
	}

	cfg := config.AppConfig.MQTT

	payload, err := json.Marshal(alert)
	if err != nil {
		return fmt.Errorf("failed to marshal alert: %v", err)
	}

	topic := fmt.Sprintf("%s/gate-%d", cfg.TopicAlert, alert.GateID)

	token := MQTTClient.Publish(topic, 1, false, payload)
	go func() {
		token.Wait()
		if token.Error() != nil {
			log.Printf("Failed to publish alert to MQTT: %v", token.Error())
		} else {
			log.Printf("Alert published to MQTT topic: %s", topic)
		}
	}()

	return nil
}

func CloseMQTT() {
	if MQTTClient != nil && MQTTClient.IsConnected() {
		MQTTClient.Disconnect(250)
		log.Println("MQTT client disconnected")
	}
}
