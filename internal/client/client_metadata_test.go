package client

import (
	"testing"

	"github.com/segmentio/kafka-go"
)

func TestBrokerInfoUsesAdvertisedMetadata(t *testing.T) {
	t.Parallel()

	got := brokerInfo(&kafka.Broker{
		ID:   3,
		Host: "broker.internal",
		Port: 19092,
		Rack: "rack-a",
	})

	if got.ID != 3 || got.Host != "broker.internal" || got.Port != 19092 || got.Rack != "rack-a" {
		t.Fatalf("unexpected broker info: %+v", got)
	}
}

func TestBrokerInfoDoesNotInventMissingPort(t *testing.T) {
	t.Parallel()

	got := brokerInfo(&kafka.Broker{Host: "broker.internal"})
	if got.Port != 0 {
		t.Fatalf("expected unavailable port to remain zero, got %d", got.Port)
	}
}
