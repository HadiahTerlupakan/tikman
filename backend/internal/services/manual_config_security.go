package services

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const redactedValue = "<redacted>"

var sensitiveConfigKey = regexp.MustCompile(`(?i)(password|passwd|secret|token|api[_-]?key|community|credential)`)
var sensitiveErrorValue = regexp.MustCompile(`(?i)((?:password|passwd|secret|token|api[_-]?key|community|credential)\s*[:=]\s*)[^\s,;]+`)
var safeConfigToken = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

// validateManualConfig restricts generic provisioning to the fields consumed by
// the generic command builders. Values must remain single CLI-safe tokens.
func validateManualConfig(config map[string]interface{}) error {
	for key, value := range config {
		switch key {
		case "bandwidth":
			if err := validateManualToken(key, value); err != nil {
				return err
			}
		case "vlan":
			vlan, err := manualInteger(key, value)
			if err != nil {
				return err
			}
			if vlan < 1 || vlan > 4094 {
				return fmt.Errorf("vlan must be in range 1-4094")
			}
		default:
			return fmt.Errorf("manual config key %q is not allowed", key)
		}
	}
	return nil
}

func manualTokenString(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 32)
	case int:
		return strconv.Itoa(typed)
	case int8:
		return strconv.FormatInt(int64(typed), 10)
	case int16:
		return strconv.FormatInt(int64(typed), 10)
	case int32:
		return strconv.FormatInt(int64(typed), 10)
	case int64:
		return strconv.FormatInt(typed, 10)
	case uint:
		return strconv.FormatUint(uint64(typed), 10)
	case uint8:
		return strconv.FormatUint(uint64(typed), 10)
	case uint16:
		return strconv.FormatUint(uint64(typed), 10)
	case uint32:
		return strconv.FormatUint(uint64(typed), 10)
	case uint64:
		return strconv.FormatUint(typed, 10)
	default:
		return ""
	}
}

func validateManualToken(key string, value interface{}) error {
	switch typed := value.(type) {
	case string:
		if typed == "" || !safeConfigToken.MatchString(typed) {
			return fmt.Errorf("%s contains unsupported characters", key)
		}
	case jsonNumber:
		if _, err := strconv.ParseFloat(string(typed), 64); err != nil {
			return fmt.Errorf("%s must be a scalar CLI token", key)
		}
	case float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return nil
	default:
		return fmt.Errorf("%s must be a scalar CLI token", key)
	}
	return nil
}

type jsonNumber = json.Number

func manualInteger(key string, value interface{}) (int, error) {
	switch typed := value.(type) {
	case string:
		if !regexp.MustCompile(`^[0-9]+$`).MatchString(typed) {
			return 0, fmt.Errorf("%s must be an integer CLI token", key)
		}
		parsed, err := strconv.Atoi(typed)
		if err != nil {
			return 0, fmt.Errorf("%s must be an integer CLI token", key)
		}
		return parsed, nil
	case float64:
		if typed != float64(int(typed)) {
			return 0, fmt.Errorf("%s must be an integer CLI token", key)
		}
		return int(typed), nil
	case int:
		return typed, nil
	case int8:
		return int(typed), nil
	case int16:
		return int(typed), nil
	case int32:
		return int(typed), nil
	case int64:
		return int(typed), nil
	case uint:
		return int(typed), nil
	case uint8:
		return int(typed), nil
	case uint16:
		return int(typed), nil
	case uint32:
		return int(typed), nil
	case uint64:
		return int(typed), nil
	default:
		return 0, fmt.Errorf("%s must be an integer CLI token", key)
	}
}

func redactManualConfig(config map[string]interface{}) map[string]interface{} {
	return redactMap(config)
}

func redactMap(input map[string]interface{}) map[string]interface{} {
	output := make(map[string]interface{}, len(input))
	for key, value := range input {
		if sensitiveConfigKey.MatchString(key) {
			output[key] = redactedValue
			continue
		}
		output[key] = redactValue(value)
	}
	return output
}

func redactValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		return redactMap(typed)
	case []interface{}:
		output := make([]interface{}, len(typed))
		for i, item := range typed {
			output[i] = redactValue(item)
		}
		return output
	default:
		return value
	}
}

func redactProvisioningError(message string, config map[string]interface{}) string {
	for key, value := range sensitiveValues(config) {
		if value != "" {
			message = strings.ReplaceAll(message, value, redactedValue)
		}
		_ = key
	}
	return sensitiveErrorValue.ReplaceAllString(message, `${1}`+redactedValue)
}

func sensitiveValues(input map[string]interface{}) map[string]string {
	values := make(map[string]string)
	for key, value := range input {
		if sensitiveConfigKey.MatchString(key) {
			if text, ok := value.(string); ok {
				values[key] = text
			}
			continue
		}
		if nested, ok := value.(map[string]interface{}); ok {
			for nestedKey, nestedValue := range sensitiveValues(nested) {
				values[nestedKey] = nestedValue
			}
		}
	}
	return values
}
