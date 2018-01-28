package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
)

const timeout = time.Second * 5

var errorTimeout = errors.New("MQTT response not received during specified time frame")
var outTopic string = os.Getenv("MQTT_OUT_TOPIC")

var client mqtt.Client

func handler(in interface{}) (out interface{}, err error) {
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}
	defer client.Disconnect(0)

	reqID := uuid.New()

	ch := make(chan interface{})
	errChan := make(chan error)

	inTopic := fmt.Sprintf("%s/response/%s", outTopic, reqID)
	token := client.Subscribe(inTopic, 2, func(client mqtt.Client, message mqtt.Message) {
		payload := message.Payload()

		var res interface{}
		if err := json.Unmarshal(payload, &res); err != nil {
			errChan <- err
			return
		}

		ch <- res
	})

	if token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}

	req := map[string]interface{}{
		"id":             reqID.String(),
		"response_topic": inTopic,
		"payload":        in,
	}

	b, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	if token := client.Publish(outTopic, 1, false, b); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}

	select {
	case err = <-errChan:
	case out = <-ch:
	case <-time.After(timeout):
		err = errorTimeout
	}

	return
}

func init() {
	clientID := os.Getenv("MQTT_CLIENT_ID")
	broker := os.Getenv("MQTT_BROKER")
	username := os.Getenv("MQTT_USERNAME")
	password := os.Getenv("MQTT_PASSWORD")

	if outTopic == "" {
		panic("Output topic should be defined")
	}

	if broker == "" {
		panic("MQTT broker should be defined")
	}

	opts := mqtt.NewClientOptions()
	opts = opts.AddBroker(broker)
	if clientID != "" {
		opts = opts.SetClientID(clientID)
	}
	if username != "" {
		opts = opts.SetUsername(username)
	}
	if password != "" {
		opts = opts.SetPassword(password)
	}
	client = mqtt.NewClient(opts)
}

func main() {
	lambda.Start(handler)
}
