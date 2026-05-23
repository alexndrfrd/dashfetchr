package postgres

import (
	"encoding/json"
	"fmt"
)

func marshalJSON(v any) ([]byte, error) {
	if v == nil {
		return []byte("{}"), nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("postgres: json marshal: %w", err)
	}
	return b, nil
}

func unmarshalJSON(data []byte, v any) error {
	if len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("postgres: json unmarshal: %w", err)
	}
	return nil
}
