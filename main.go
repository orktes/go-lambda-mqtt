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
	"github.com/orktes/go-lambda-mqtt/structs"
)

const timeout = time.Second * 5

var errorTimeout = errors.New("MQTT response not received during specified time frame")
var outTopic string = os.Getenv("MQTT_OUT_TOPIC")

var client mqtt.Client

func handler(in json.RawMessage) (out json.RawMessage, err error) {
	if outTopic == "" {
		panic("Output topic should be defined")
	}

	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}
	defer client.Disconnect(0)

	reqID := uuid.New()

	ch := make(chan json.RawMessage)
	errChan := make(chan error)

	inTopic := fmt.Sprintf("%s/response/%s", outTopic, reqID)
	token := client.Subscribe(inTopic, 2, func(client mqtt.Client, message mqtt.Message) {
		payload := message.Payload()
		ch <- payload
	})

	if token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}

	req := structs.Request{
		ID:      reqID.String(),
		Topic:   inTopic,
		Payload: in,
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

	if token := client.Unsubscribe(inTopic); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}

	return
}

func init() {
	clientID := os.Getenv("MQTT_CLIENT_ID")
	broker := os.Getenv("MQTT_BROKER")
	username := os.Getenv("MQTT_USERNAME")
	password := os.Getenv("MQTT_PASSWORD")

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
