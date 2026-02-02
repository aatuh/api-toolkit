package cedar

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"time"

	cedarcore "github.com/cedar-policy/cedar-go"
)

// RecordFromMap converts a map into a Cedar record using supported value types.
func RecordFromMap(input map[string]any) (cedarcore.Record, error) {
	if input == nil {
		return cedarcore.Record{}, nil
	}
	out := make(cedarcore.RecordMap, len(input))
	for k, v := range input {
		if strings.TrimSpace(k) == "" {
			return cedarcore.Record{}, errors.New("record key is empty")
		}
		value, err := ValueFromAny(v)
		if err != nil {
			return cedarcore.Record{}, fmt.Errorf("record[%s]: %w", k, err)
		}
		out[cedarcore.String(k)] = value
	}
	return cedarcore.NewRecord(out), nil
}

// ValueFromAny converts a Go value into a Cedar value.
func ValueFromAny(input any) (cedarcore.Value, error) {
	if input == nil {
		return nil, errors.New("value is nil")
	}
	if v, ok := input.(cedarcore.Value); ok {
		return v, nil
	}
	switch v := input.(type) {
	case string:
		return cedarcore.String(v), nil
	case []byte:
		return cedarcore.String(string(v)), nil
	case bool:
		return cedarcore.Boolean(v), nil
	case int:
		return cedarcore.Long(v), nil
	case int8:
		return cedarcore.Long(v), nil
	case int16:
		return cedarcore.Long(v), nil
	case int32:
		return cedarcore.Long(v), nil
	case int64:
		return cedarcore.Long(v), nil
	case uint:
		if v > math.MaxInt64 {
			return nil, errors.New("unsigned value overflows cedar long")
		}
		return cedarcore.Long(v), nil
	case uint8:
		return cedarcore.Long(v), nil
	case uint16:
		return cedarcore.Long(v), nil
	case uint32:
		return cedarcore.Long(v), nil
	case uint64:
		if v > math.MaxInt64 {
			return nil, errors.New("unsigned value overflows cedar long")
		}
		return cedarcore.Long(v), nil
	case float32:
		dec, err := cedarcore.NewDecimalFromFloat(v)
		if err != nil {
			return nil, err
		}
		return dec, nil
	case float64:
		dec, err := cedarcore.NewDecimalFromFloat(v)
		if err != nil {
			return nil, err
		}
		return dec, nil
	case time.Time:
		return cedarcore.NewDatetime(v), nil
	case time.Duration:
		return cedarcore.NewDuration(v), nil
	case map[string]any:
		rec, err := RecordFromMap(v)
		if err != nil {
			return nil, err
		}
		return rec, nil
	case map[string]cedarcore.Value:
		return cedarcore.NewRecord(stringKeyedRecord(v)), nil
	}
	rv := reflect.ValueOf(input)
	if rv.Kind() == reflect.Slice {
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			return cedarcore.String(string(rv.Bytes())), nil
		}
		values := make([]cedarcore.Value, 0, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			val, err := ValueFromAny(rv.Index(i).Interface())
			if err != nil {
				return nil, fmt.Errorf("set[%d]: %w", i, err)
			}
			values = append(values, val)
		}
		return cedarcore.NewSet(values...), nil
	}
	return nil, fmt.Errorf("unsupported value type %T", input)
}
