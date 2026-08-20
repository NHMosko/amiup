package main

// TODO:
// DONE Check json body before dealing with it (empty post bug)

// - Save the data (banco de dados)
//   º Services
//   DONE Salvar dados das requisições (quando, response, codigo, duracao entre conectar e responder)
// 	 º table REQUISITIONS: id UUID, service_id INT, when DATETIME, status NUMBER, response.body STRING, duration NUMBER
//
// - Load the data

// DONE ENV variables (ler do ambiente)
//   DONE Addr (port, ip) -> de onde o amiup vai receber chamados
//   DONE API KEY -> receber e verificar se as requisições a contem no header verification (Bearer Token) auth
//   DONE Allow Insecure Target (aceitar http ou só https)

// DONE Usar Go Tool Air (hot reload)


// - Entender o bug de não ter os dados dos serviços nos outros endpoints

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
	_ "github.com/lib/pq"
)

type Service struct {
	Name           string `json:"name"`
	Timeout        int    `json:"timeout"`
	URL            string `json:"url"`
	Heartbeat      int    `json:"heartbeat"`
	Strikes        int    `json:"strikes"`
	DiscordWebhook string `json:"discord_webhook"`
	wasDown        bool
	whenDown       time.Time
	strikeCounter  int
	totalCounter   int
	downCounter    int
}

var services []Service

type Requisition struct {
	when          time.Time
	response_body string
	status        int
	duration      time.Duration
}

type RemovalRequest struct {
	Indexes []int `json:"indexes"`
}

var expectedKey string
var allowedAddr string
var allowInsecureTarget bool

// Streak Test
func main() {
	expectedKey             = os.Getenv("API_KEY")
	allowedAddr             = os.Getenv("REMOTE_ADDR")
	allowInsecureTargetStr := os.Getenv("ALLOW_INSECURE_TARGET")

	var err error
	allowInsecureTarget, err = strconv.ParseBool(allowInsecureTargetStr)
	if err != nil {
		fmt.Printf("Invalid ALLOW_INSECURE_TARGET config: %s - Err: %v", allowInsecureTargetStr, err)
		return
	}

	for _, service := range services {
		go check(&service) // eu sinto que checar a coisa em paralelo está me impedindo de acessar os valores certos
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /services", addNewService)
	mux.HandleFunc("GET /services", listServices)
	mux.HandleFunc("POST /remove-service", removeService) // não funciona

	server := http.Server{
		Handler: mux,
		Addr:    ":8082",
	}
	log.Printf("Server on port %v", server.Addr)
	log.Fatal(server.ListenAndServe())
}

func addNewService(w http.ResponseWriter, r *http.Request) {
	//fmt.Printf("Authorization: %q\n", r.Header.Get("Authorization"))
	//fmt.Printf("Remote Address: %v", r.RemoteAddr)

	addr, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil || addr != allowedAddr {
		respondWithError(w, http.StatusForbidden, "Invalid remote address")
		return
	}

	key, err := GetAPIKey(r.Header)
	if err != nil || key != expectedKey {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	serv := Service{}
	if !decodeInput(w, r, &serv) {
		return
	}
	if missingRequiredFields(serv) {
		log.Println("Missing Required Fields")
		respondWithError(w, http.StatusBadRequest, "Missing Required Fields")
		return
	}
	services = append(services, serv)

	go check(&serv)
	respondWithJSON(w, http.StatusCreated, serv)
	log.Printf("New service added: %v", serv.Name)
}

func listServices(w http.ResponseWriter, r *http.Request) {
	// listar serviços e porcentagens
	for _, service := range services {
		log.Println(service)
	}
	respondWithJSON(w, http.StatusOK, "Services.")
}

func removeService(w http.ResponseWriter, r *http.Request) {
	if len(services) == 0 {
		respondWithError(w, http.StatusBadRequest, "No services to remove")
		return
	}
	allowedAddr := os.Getenv("REMOTE_ADDR")
	addr, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil || addr != allowedAddr {
		respondWithError(w, http.StatusForbidden, "Invalid remote address")
		return
	}

	expectedKey := os.Getenv("API_KEY")
	key, err := GetAPIKey(r.Header)
	if err != nil || key != expectedKey {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	remReq := RemovalRequest{}
	if !decodeInput(w, r, &remReq) {
		return
	}

	for _, v := range remReq.Indexes {
		if len(services) < v {
			respondWithError(w, http.StatusBadRequest, "Service inexistent")
			return
		}
	}
	var newSlice []Service
	for i := range services {
		if !slices.Contains(remReq.Indexes, i) {
			newSlice = append(newSlice, services[i])
		}
	}
	services = newSlice
	log.Printf("%v", services[0])
	respondWithJSON(w, http.StatusAccepted, "Services removed")
}

func check(service *Service) {
	if service == nil {
		return
	}

	if !allowInsecureTarget {
		url, err := url.Parse(service.URL)
		if err != nil {
			log.Println(err)
			return
		}
		if url.Scheme == "http" {
			log.Println("Connection Type Not Allowed")
		}
	}

	ticker := time.NewTicker(time.Duration(service.Heartbeat) * time.Second)
	client := http.Client{
		Timeout: time.Duration(service.Timeout) * time.Second, //timeout <= heartbeat/2
	}
	for {
		//mudar ticker pra delay pós requisição
		<-ticker.C

		time_check := time.Now()
		ok, req := service.checkAvailability(client)
		if !ok {
			//exponential backoff (~random) for strikes
		}
		req.duration = time.Since(time_check)

		//fmt.Println(req.when)
		//fmt.Println(len(req.response_body))
		//fmt.Println(req.status)
		//fmt.Println(req.duration)
	}
}

//melhorar/detalhar logs

func (s *Service) checkAvailability(client http.Client) (bool, Requisition) {
	defer func() {
		s.totalCounter++
		fmt.Printf("Uptime percentage: %.2f%% (%v/%v)\n\n", 100-((float64(s.downCounter)*100)/float64(s.totalCounter)), s.totalCounter-s.downCounter, s.totalCounter)
	}()

	requisition := Requisition{}
	res, err := client.Get(s.URL)

	if err != nil {
		fmt.Println(err)
	} else {
		defer res.Body.Close()
		body, err := io.ReadAll(res.Body)
		if err != nil {
			log.Println(err)
		}
		requisition.response_body = string(body)
		requisition.when = time.Now()
		requisition.status = res.StatusCode

		if res.StatusCode >= 200 && res.StatusCode <= 299 {
			log.Println(s.Name, s.URL, "is up.")
			s.strikeCounter = 0
			if s.wasDown {
				s.wasDown = false
				notifyDiscord(fmt.Sprintf("✅ Your service %v is back up! :) - Downtime: %v", s.URL, time.Since(s.whenDown)), s.DiscordWebhook)
				s.whenDown = time.Time{}
			}
			return true, requisition
		}
	}

	s.strikeCounter++
	s.downCounter++
	log.Printf("%s seems down - Strike %v/%v\n", s.Name, s.strikeCounter, s.Strikes)
	if s.strikeCounter == s.Strikes {
		s.strikeCounter = 0
		notifyDiscord(fmt.Sprintf("❌ Your service %v went down :(", s.URL), s.DiscordWebhook)
		if !s.wasDown {
			s.wasDown = true
			s.whenDown = time.Now()
		}
	}
	return false, requisition
}

func notifyDiscord(message string, discordWebhook string) {
	if discordWebhook == "" {
		log.Printf("No Discord configured - %s", message)
		return
	}

	payload := fmt.Sprintf(
		`{"content": "%v\n time: %v"}`,
		message, time.Now(),
	)
	reader := strings.NewReader(payload)
	req, err := http.NewRequest(
		"POST",
		discordWebhook,
		reader,
	)
	if err != nil {
		fmt.Println("error on req:", err)
	}

	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("couldn't notify:", err)
		return
	}
	defer res.Body.Close()

	body, _ := io.ReadAll(res.Body)

	if res.StatusCode != 204 {
		fmt.Println("discord status:", res.StatusCode)
		fmt.Println("discord body:", string(body))
	} else {
		fmt.Println("Discord notification sent!")
	}
}

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

func missingRequiredFields(service Service) bool {
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

// Old code

//discorddowndata = {
//username: "amiup",
//embeds: [{
//title: "❌ Your service " + monitorJSON["name"] + " went down. ❌",
//color: 16711680,
//timestamp: heartbeatJSON["time"],
//fields: [
//{
//name: "Service Name",
//value: monitorJSON["name"],
//},
//...(!notification.disableUrl ? [{
//name: monitorJSON["type"] === "push" ? "Service Type" : "Service URL",
////value: this.extractAddress(monitorJSON),
//}] : []),
//{
//name: `Time (${heartbeatJSON["timezone"]})`,
////value: heartbeatJSON["localDateTime"],
//},
//{
//name: "Error",
//value: heartbeatJSON["msg"] == null ? "N/A" : heartbeatJSON["msg"],
//},
//],
//}],
//////};

// Example Usage
/*
curl -H "Authorization: ApiKey 1234" localhost:8082/services --data '{"name": "Google", "timeout": 10, "url": "https://google.com", "heartbeat": 5, "strikes": 3, "discord_webhook": ""}'
*/
