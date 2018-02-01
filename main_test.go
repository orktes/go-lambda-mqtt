package main

import (
	"encoding/json"
	"strings"
	"testing"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/orktes/go-lambda-mqtt/structs"
)

func TestHandler(t *testing.T) {
	subs := make(chan struct {
		topic    string
		callback mqtt.MessageHandler
	})
	pubs := make(chan struct {
		topic   string
		payload []byte
	})
	newClient = func(opts *mqtt.ClientOptions) mqtt.Client {
		return &mockClient{subs, pubs}
	}
	outTopic = "foo"

	res := make(chan []byte)
	go func() {
		r, err := handler([]byte("\"somepayload data\""))
		if err != nil {
			t.Error(err)
		}

		res <- r
	}()

	s := <-subs
	if !strings.HasPrefix(s.topic, "foo/response/") {
		t.Error("Subscribed to wrong topic")
	}

	p := <-pubs
	if p.topic != "foo" {
		t.Error("Published to wrong topic")
	}

	req := &structs.Request{}
	if err := json.Unmarshal(p.payload, req); err != nil {
		t.Error(err)
	}

	if req.Topic != s.topic {
		t.Error("Request cotained wrong response topic")
	}

	if string(req.Payload) != "\"somepayload data\"" {
		t.Error("Wrong request data received", string(req.Payload))
	}

	s.callback(nil, mockMessage{topic: req.Topic, payload: []byte("\"some response\"")})

	if string(<-res) != "\"some response\"" {
		t.Error("Wrong response received")
	}
}
