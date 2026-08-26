package main

// Criando o banco de dados com base em:
// https://www.boot.dev/lessons/0820daf4-4006-425a-a50c-f45c0eb97d06

// TODO:
// DONE Check json body before dealing with it (empty post bug)
// DONE ENV variables (ler do ambiente)
//   DONE Addr (port, ip) -> de onde o amiup vai receber chamados
//   DONE API KEY -> receber e verificar se as requisições a contem no header verification (Bearer Token) auth
//   DONE Allow Insecure Target (aceitar http ou só https)
// DONE Usar Go Tool Air (hot reload)
// DONE Entender o bug de não ter os dados dos serviços nos outros endpoints

// - Load the data
// - Save the data (banco de dados)
//   DONE Services
//   DONE Salvar dados das requisições (quando, response, codigo, duracao entre conectar e responder)
// 	 º table REQUISITIONS: id UUID, service_id UUID, when DATETIME, status NUMBER, response.body STRING, duration NUMBER

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/NHMosko/amiup/internal/database"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

type CreateService struct {
	Name           string `json:"name"`
	Timeout        int    `json:"timeout"`
	URL            string `json:"url"`
	Heartbeat      int    `json:"heartbeat"`
	Strikes        int    `json:"strikes"`
	DiscordWebhook string `json:"discord_webhook"`
}

type Service struct {
	DB_ID          uuid.UUID
	Name           string
	Timeout        int
	URL            string
	Heartbeat      int
	Strikes        int
	DiscordWebhook string
	wasDown        bool
	whenDown       sql.NullTime
	strikeCounter  int
	totalCounter   int
	downCounter    int
}

type Requisition struct {
	when          time.Time
	response_body []byte
	status        int
	duration      time.Duration
}

type RemovalRequest struct {
	Indexes []int `json:"indexes"`
}

type Config struct {
	db *database.Queries
}

var expectedKey string
var allowedAddr string
var allowInsecureTarget bool
var cfg Config

func main() {
	// setup
	expectedKey = os.Getenv("API_KEY")
	allowedAddr = os.Getenv("REMOTE_ADDR")
	allowInsecureTargetStr := os.Getenv("ALLOW_INSECURE_TARGET")
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		fmt.Println(err)
		return
	}
	dbQueries := database.New(db)
	cfg = Config{
		db: dbQueries,
	}
	allowInsecureTarget, err = strconv.ParseBool(allowInsecureTargetStr)
	if err != nil {
		fmt.Printf("Invalid ALLOW_INSECURE_TARGET config: '%s' - Err: %v", allowInsecureTargetStr, err)
		return
	}
	dbServices, err := cfg.db.GetServices(context.Background())
	if err != nil {
		log.Printf("Failed to get services from database: %v\n", err)
		return
	}

	// action
	for _, service := range dbServices {
		go check(service.ID)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /services", middlewareAuth(addNewService))
	mux.HandleFunc("POST /services/{serviceID}", middlewareAuth(editService))
	mux.HandleFunc("GET /services", listServices)
	mux.HandleFunc("GET /services/{serviceID}", getService)
	mux.HandleFunc("GET /services/delete/{serviceID}", middlewareAuth(deleteService))
	mux.HandleFunc("GET /requisitions", listRequisitions)
	server := http.Server{
		Handler: mux,
		Addr:    ":8082",
	}

	log.Printf("Server on port %v", server.Addr)
	log.Fatal(server.ListenAndServe())
}

func addNewService(w http.ResponseWriter, r *http.Request) {
	serv := CreateService{}
	if !decodeInput(w, r, &serv) {
		return
	}
	if missingRequiredFields(serv) {
		log.Println("Missing Required Fields")
		respondWithError(w, http.StatusBadRequest, "Missing Required Fields")
		return
	}

	addedService, err := cfg.db.CreateService(r.Context(), database.CreateServiceParams{
		Name:           serv.Name,
		Url:            serv.URL,
		Timeout:        int32(serv.Timeout),
		Heartbeat:      int32(serv.Heartbeat),
		Strikes:        int32(serv.Strikes),
		DiscordWebhook: serv.DiscordWebhook,
	})
	if err != nil {
		log.Printf("Failed to add service to database: %v\n", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to add service to database")
		return
	}

	go check(addedService.ID)
	respondWithJSON(w, http.StatusCreated, addedService)
	log.Printf("New service added: %v", serv.Name)
}

func editService(w http.ResponseWriter, r *http.Request) {
	serviceIDString := r.PathValue("serviceID")
	serviceID, err := uuid.Parse(serviceIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid service ID")
		return
	}

	baseService, err := cfg.db.GetService(r.Context(), serviceID)
	if err != nil {
		log.Printf("Failed to get service from database: %v\n", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to get service from database")
		return
	}

	serv := CreateService{}
	if !decodeInput(w, r, &serv) {
		return
	}
	if serv.Name == "" {
		serv.Name = baseService.Name
	}
	if serv.URL == "" {
		serv.URL = baseService.Url
	}
	if serv.DiscordWebhook == "" {
		serv.DiscordWebhook = baseService.DiscordWebhook
	}
	if serv.Timeout == 0 {
		serv.Timeout = int(baseService.Timeout)
	}
	if serv.Heartbeat == 0 {
		serv.Heartbeat = int(baseService.Heartbeat)
	}
	if serv.Strikes == 0 {
		serv.Strikes = int(baseService.Strikes)
	}

	err = cfg.db.EditService(r.Context(), database.EditServiceParams{
		ID:             serviceID,
		Name:           serv.Name,
		Url:            serv.URL,
		DiscordWebhook: serv.DiscordWebhook,
		Timeout:        int32(serv.Timeout),
		Heartbeat:      int32(serv.Heartbeat),
		Strikes:        int32(serv.Strikes),
	})
	if err != nil {
		log.Printf("Failed to add service to database: %v\n", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to add service to database")
		return
	}

	respondWithJSON(w, http.StatusAccepted, "Service edited.")
	log.Printf("%v edited", serv.Name)
}

func listServices(w http.ResponseWriter, r *http.Request) {
	// listar serviços e porcentagens
	services, err := cfg.db.GetServices(r.Context())
	if err != nil {
		log.Printf("Failed to get services from database: %v\n", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to get services from database")
		return
	}

	respondWithJSON(w, http.StatusOK, services)
}

func getService(w http.ResponseWriter, r *http.Request) {
	serviceIDString := r.PathValue("serviceID")
	serviceID, err := uuid.Parse(serviceIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid service ID")
		return
	}
	service, err := cfg.db.GetService(r.Context(), serviceID)
	if err != nil {
		log.Printf("Failed to get service from database: %v\n", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to get service from database")
		return
	}

	respondWithJSON(w, http.StatusOK, service)
}

func deleteService(w http.ResponseWriter, r *http.Request) {
	serviceIDString := r.PathValue("serviceID")
	serviceID, err := uuid.Parse(serviceIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid service ID")
		return
	}
	err = cfg.db.DeleteService(r.Context(), serviceID)
	if err != nil {
		log.Printf("Failed to delete service from database: %v\n", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to delete service from database")
		return
	}

	respondWithJSON(w, http.StatusOK, "Deleted")
}

func decode_service(service database.Service) Service {
	return Service{
		DB_ID:          service.ID,
		Name:           service.Name,
		Timeout:        int(service.Timeout),
		URL:            service.Url,
		Heartbeat:      int(service.Heartbeat),
		Strikes:        int(service.Strikes),
		DiscordWebhook: service.DiscordWebhook,
		wasDown:        service.WasDown,
		whenDown:       service.WhenDown,
		strikeCounter:  int(service.StrikeCounter),
		totalCounter:   int(service.TotalCounter),
		downCounter:    int(service.DownCounter),
	}
}

func check(service_id uuid.UUID) {
	for {
		service, err := cfg.db.GetService(context.Background(), service_id)
		if err != nil {
			log.Printf("Failed to get service from db or it doesn't exist anymore")
			return
		}
		if !allowInsecureTarget {
			url, err := url.Parse(service.Url)
			if err != nil {
				log.Println(err)
				return
			}
			if url.Scheme == "http" {
				log.Printf("Connection Type Not Allowed For %v", service.Url)
				return
			}
		}

		serv := decode_service(service)
		ticker := time.NewTicker(time.Duration(serv.Heartbeat) * time.Second)
		client := http.Client{
			Timeout: time.Duration(serv.Timeout) * time.Second, //timeout <= heartbeat/2
		}
		<-ticker.C

		time_check := time.Now()
		up, req := serv.checkAvailability(client)
		if !up {
			// service is down
		}
		req.duration = time.Since(time_check)
		_, err = cfg.db.CreateRequisition(context.Background(), database.CreateRequisitionParams{
			ServiceID:    serv.DB_ID,
			RequestTime:  req.when,
			Status:       int32(req.status),
			ResponseBody: req.response_body,
			Duration:     int32(req.duration),
		})
		if err != nil {
			log.Printf("Adding req to db failed: %v", err)
		}
	}
}

//melhorar/detalhar logs

func (s *Service) checkAvailability(client http.Client) (bool, Requisition) {
	defer func() {
		s.totalCounter++
		cfg.db.UpdateService(context.Background(), database.UpdateServiceParams{
			ID:            s.DB_ID,
			WasDown:       s.wasDown,
			WhenDown:      s.whenDown,
			StrikeCounter: int32(s.strikeCounter),
			TotalCounter:  int32(s.totalCounter),
			DownCounter:   int32(s.downCounter),
		})
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
		requisition.response_body = body
		requisition.when = time.Now()
		requisition.status = res.StatusCode

		if res.StatusCode >= 200 && res.StatusCode <= 299 {
			log.Println(s.Name, s.URL, "is up.")
			s.strikeCounter = 0
			if s.wasDown {
				s.wasDown = false
				notifyDiscord(fmt.Sprintf("✅ Your service %v is back up! :) - Downtime: %v", s.URL, time.Since(s.whenDown.Time)), s.DiscordWebhook)
				s.whenDown.Valid = false
			}
			return true, requisition
		}
	}

	s.strikeCounter++
	s.downCounter++
	log.Printf("%s seems down - Strike %v/%v\n", s.Name, s.strikeCounter, s.Strikes)
	if s.strikeCounter == s.Strikes {
		s.strikeCounter = 0
		if !s.wasDown {
			s.wasDown = true
			s.whenDown.Time = time.Now()
			s.whenDown.Valid = true
		}
		notifyDiscord(fmt.Sprintf("❌ Your service %v went down :(", s.URL), s.DiscordWebhook)
		notifyDiscord(fmt.Sprintf("Down when: %v", s.whenDown.Time), s.DiscordWebhook)
	}
	return false, requisition
}

func middlewareAuth(next func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return (func(w http.ResponseWriter, r *http.Request) {
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
		next(w, r)
	})
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

func listRequisitions(w http.ResponseWriter, r *http.Request) {
	requisitions, err := cfg.db.GetRequisitions(r.Context())
	if err != nil {
		log.Printf("Failed to get requisitions from database: %v\n", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to get requisitions from database")
		return
	}

	respondWithJSON(w, http.StatusOK, requisitions)
}

// Example Usage
/*
curl -H "Authorization: ApiKey 1234" localhost:8082/services --data '{"name": "Google", "timeout": 10, "url": "https://google.com", "heartbeat": 5, "strikes": 3, "discord_webhook": ""}'
*/
