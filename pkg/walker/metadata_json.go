package walker

import (
	"bytes"
	"encoding/json"
)

// UnmarshalJSON preserves the distinction between an omitted body and explicit
// JSON null, while retaining strict nested-field checking and numeric precision.
func (r *Route) UnmarshalJSON(data []byte) error {
	type wire Route
	var value wire
	if err := decodeBodyMetadata(data, &value, &value.Body); err != nil {
		return err
	}
	*r = Route(value)
	return nil
}

// UnmarshalJSON gives named variants the same body semantics as routes.
func (r *RequestVariant) UnmarshalJSON(data []byte) error {
	type wire RequestVariant
	var value wire
	if err := decodeBodyMetadata(data, &value, &value.Body); err != nil {
		return err
	}
	*r = RequestVariant(value)
	return nil
}

func decodeBodyMetadata(data []byte, target any, body *any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	// Standard JSON field matching is case-insensitive. Decode into an auxiliary
	// field to preserve that behavior even if the input uses "Body".
	var presence struct {
		Body json.RawMessage `json:"body"`
	}
	if err := json.Unmarshal(data, &presence); err != nil {
		return err
	}
	if bytes.Equal(bytes.TrimSpace(presence.Body), []byte("null")) {
		*body = json.RawMessage("null")
	}
	return nil
}
