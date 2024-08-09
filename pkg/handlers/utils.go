package handlers

import (
	"fmt"
	"net/url"
	"strconv"
)

func parseIntQueryParam(params url.Values, paramName string, defaultValue int) (int, error) {
	param := params.Get(paramName)

	if param == "" {
		return defaultValue, nil
	}

	value, err := strconv.Atoi(param)

	if err != nil {
		return 0, fmt.Errorf("Failed to parse %s: %w", paramName, err)
	}

	return value, nil
}

func parseBoolQueryParam(params url.Values, paramName string, defaultValue bool) (bool, error) {
	param := params.Get(paramName)

	if param == "" {
		return defaultValue, nil
	}

	value, err := strconv.ParseBool(param)

	if err != nil {
		return false, fmt.Errorf("Failed to parse %s: %w", paramName, err)
	}

	return value, nil
}
