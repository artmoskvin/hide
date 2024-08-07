package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gorilla/mux"
)

func getProjectID(r *http.Request) (string, error) {
	return getPathValue(r, "id")
}

func getFilePath(r *http.Request) (string, error) {
	return getPathValue(r, "path")
}

func getPathValue(r *http.Request, key string) (string, error) {
	vars := mux.Vars(r)
	value, ok := vars[key]
	if !ok || value == "" {
		return "", errors.New("invalid path value")
	}

	return value, nil
}

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
