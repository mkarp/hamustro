package dialects

import (
	"../payload"
	"bytes"
	"encoding/json"
	"time"
)

// Converts EPOCH timestamp to isoformat string
func ConvertIsoformat(at uint64) string {
	return time.Unix(int64(at), 0).Format("2006-01-02T15:04:05")
}

// Single event
type Event struct {
	DeviceID       string `json:"device_id"`
	ClientID       string `json:"client_id"`
	Session        string `json:"session"`
	Nr             uint32 `json:"nr"`
	SystemVersion  string `json:"system_version"`
	ProductVersion string `json:"product_version"`
	At             string `json:"at"`
	Event          string `json:"event"`
	System         string `json:"system,omitempty"`
	ProductGitHash string `json:"product_git_hash,omitempty"`
	UserID         uint32 `json:"user_id,omitempty"`
	IP             string `json:"ip,omitempty"`
	Parameters     string `json:"parameters,omitempty"`
	IsTesting      bool   `json:"is_testing"`
}

// Creates a new event based on the collection and a single payload
func NewEvent(meta *payload.Collection, payload *payload.Payload) *Event {
	return &Event{
		DeviceID:       meta.GetDeviceId(),
		ClientID:       meta.GetClientId(),
		Session:        meta.GetSession(),
		Nr:             payload.GetNr(),
		SystemVersion:  meta.GetSystemVersion(),
		ProductVersion: meta.GetProductVersion(),
		At:             ConvertIsoformat(payload.GetAt()),
		Event:          payload.GetEvent(),
		System:         meta.GetSystem(),
		ProductGitHash: meta.GetProductGitHash(),
		UserID:         payload.GetUserId(),
		IP:             payload.GetIp(),
		Parameters:     payload.GetParameters(),
		IsTesting:      payload.GetIsTesting()}
}

// Dumps the Event into a JSON string
func (event *Event) GetJSONMessage() (string, error) {
	b := new(bytes.Buffer)
	if err := json.NewEncoder(b).Encode(event); err != nil {
		return "", err
	}
	return b.String(), nil
}