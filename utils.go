package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
)

func decodeInput(w http.ResponseWriter, r *http.Request, out any) bool {
	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()

	err := decoder.Decode(out)
	if err != nil {
		log.Printf("Error decoding: %s", err)
		respondWithError(w, http.StatusInternalServerError, "Something went wrong")
		return false
	}
	return true
}

func missingRequiredFields(service CreateService) bool {
	if service.Name == "" {
		return true
	}
	if service.URL == "" {
		return true
	}
	if service.Timeout == 0 {
		return true
	}
	if service.Heartbeat == 0 {
		return true
	}
	if service.Strikes == 0 {
		return true
	}
	return false
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	type errorVal struct {
		Error string `json:"error"`
	}

	retErr := errorVal{Error: message}

	errDat, err := json.Marshal(retErr)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(500)
		return
	}

	w.WriteHeader(code)
	w.Header().Set("Content-Type", "application/json")
	w.Write(errDat)
}

func respondWithJSON(w http.ResponseWriter, code int, rawData any) {
	data, err := json.Marshal(rawData)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(500)
		w.Write(data)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(data)
}

// Expected Header: Authorization: ApiKey KEY_VALUE
func GetAPIKey(headers http.Header) (string, error) {
	authHeader := headers.Get("Authorization")
	if authHeader == "" {
		return "", errors.New("authorization header is missing")
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "ApiKey" {
		return "", errors.New("invalid authorization format")
	}

	return parts[1], nil
}
