package models

import (
	"fmt"

	"github.com/nbd-wtf/go-nostr"
	"github.com/tidwall/gjson"
)

type PubSubEnvelope struct {
	ClientID int64
	Result   chan nostr.Envelope
	Event    nostr.Envelope
}

type RawEventEnvelope struct {
	SubscriptionID *string
	RawEvent       []byte
}

func (r *RawEventEnvelope) Label() string {
	return "EVENT"
}

func (r *RawEventEnvelope) String() string {
	str, _ := r.MarshalJSON()
	return string(str)
}

func (v *RawEventEnvelope) UnmarshalJSON(data []byte) error {
	r := gjson.ParseBytes(data)
	arr := r.Array()
	switch len(arr) {
	case 2:
		v.RawEvent = []byte(arr[1].Raw)
		return nil
	case 3:
		v.SubscriptionID = &arr[1].Str
		v.RawEvent = []byte(arr[2].Raw)
		return nil
	default:
		return fmt.Errorf("failed to decode EVENT envelope")
	}
}

func (v RawEventEnvelope) MarshalJSON() ([]byte, error) {
	totalLength := 14 + len(*v.SubscriptionID) + len(v.RawEvent)
	result := make([]byte, 0, totalLength)
	result = append(result, `["`...)
	result = append(result, v.Label()...)
	result = append(result, `", "`...)
	result = append(result, *v.SubscriptionID...)
	result = append(result, `", `...)
	result = append(result, v.RawEvent...)
	result = append(result, "]"...)
	return result, nil
}
