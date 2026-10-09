package usecases

import "encoding/json"

// MarshalCompactJSONString renders v as compact JSON, which costs the model
// fewer tokens than indented JSON.
func MarshalCompactJSONString(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "{\"error\":\"failed to marshal JSON\"}"
	}
	return string(data)
}
