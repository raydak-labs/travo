package vpnstatus

import (
	"maps"
	"testing"
	"time"
)

func TestClassifyHandshakeBoundary(t *testing.T) {
	observation, _ := fixtureObservation(t, "connected")
	newest := observation.Handshakes[0].At
	for _, tc := range []struct {
		age   int64
		state State
	}{
		{age: 179, state: Connected},
		{age: 180, state: NoHandshake},
	} {
		t.Run(string(tc.state), func(t *testing.T) {
			now := newest.Add(time.Duration(tc.age) * time.Second)
			got, err := Classify(observation, now)
			if err != nil {
				t.Fatal(err)
			}
			if got.State != tc.state || got.HandshakeAgeSeconds == nil || *got.HandshakeAgeSeconds != tc.age {
				t.Fatalf("got %+v", got)
			}
		})
	}
}

func TestClassifyRejectsUnknownDisabledOption(t *testing.T) {
	observation, now := fixtureObservation(t, "disabled")
	section := *observation.Section
	section.Options = maps.Clone(observation.Section.Options)
	section.Options["disabled"] = "maybe"
	observation.Section = &section
	if _, err := Classify(observation, now); err == nil {
		t.Fatal("accepted unknown disabled value")
	}
}

func TestClassifyRejectsHandshakeAfterNow(t *testing.T) {
	observation, _ := fixtureObservation(t, "connected")
	now := observation.Handshakes[0].At.Add(-time.Second)
	if _, err := Classify(observation, now); err == nil {
		t.Fatal("accepted negative handshake age")
	}
}
