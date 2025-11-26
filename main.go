package main

import (
	"encoding/json"
	//"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	//"os"
	"strings"
	"time"
)

type Service struct {
	Name           string      `json:"name"`
	Timeout        int         `json:"timeout"`
	URL            string      `json:"url"`
	Heartbeat      int         `json:"heartbeat"`
	Strikes        int         `json:"strikes"`
	DiscordWebhook string      `json:"discord_webhook"`
	wasDown        bool
	whenDown       time.Time
	strikeCounter  int
	totalCounter   int
	downCounter    int
}

var services []Service


func main() {
	//if len(os.Args) <= 1 {
	//fmt.Println("Usage: amiup -url=\"example.com\" [-flag=value]\namiup -h for more info")
	//return
	//}
	//url := flag.String("url", "", "the target url")
	//strikes := flag.Int("r", 0, "number of strikes before notification")
	//heartbeat := flag.Int("hb", 30, "heartbeat, the number of seconds between checks")
	//timeout := flag.Int("timeout", 5, "seconds before http timeout")
	//discordWebhook := flag.String("webhook", "", "your discord webhook url")
	//flag.Parse()

	//services = append(services, Service{
	//	Name:           "Criador de Aulas",
	//	Timeout:        5,
	//	URL:            "http://localhost:3000",
	//	Heartbeat:      10,
	//	Strikes:        3,
	//	DiscordWebhook: "https://discord.com/api/webhooks/1442591494908284938/dudIfRrKvBEyfNiKq9aGNCKpEOl7Be1kigWMbYHFD46IKP9DEU9pd2Yj4s4I-cLxDZpL",
	//})

	for _, service := range services {
		go check(service)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /services", addNewService)
	mux.HandleFunc("GET /services", listServices)

	server := http.Server{
		Handler: mux,
		Addr:    ":8082",
	}
	log.Println("Server on")
	log.Fatal(server.ListenAndServe())
}

func addNewService(w http.ResponseWriter, r *http.Request) {
	serv := Service{}
	decodeInput(w, r, &serv)
	services = append(services, serv)

	go check(serv)
	respondWithJSON(w, http.StatusCreated, serv)
}

func listServices(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func check(service Service) {
	ticker := time.NewTicker(time.Duration(service.Heartbeat) * time.Second)
	client := http.Client{
		Timeout: time.Duration(service.Timeout) * time.Second,
	}
	for {
		<-ticker.C
		service.checkAvailability(client)
	}
}

func (s *Service) checkAvailability(client http.Client) {
	defer func() {s.totalCounter++; fmt.Printf("Uptime percentage: %.2f%%\n\n", 100 - ((float64(s.downCounter) * 100) / float64(s.totalCounter)))}()

	for range 3 {
		res, err := client.Get(s.URL)
		if err != nil {
			fmt.Println(err)
			continue
		}
		if res.StatusCode >= 200 && res.StatusCode <= 299 {
			log.Println(s.Name, s.URL, "is up.")
			s.strikeCounter = 0
			if s.wasDown {
				s.wasDown = false
				notifyDiscord(fmt.Sprintf("✅ Your service %v is back up! :) - Downtime: %v", s.URL, time.Since(s.whenDown)), s.DiscordWebhook)
				s.whenDown = time.Time{}
			}
			return
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

func decodeInput(w http.ResponseWriter, r *http.Request, out any) {
	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()

	err := decoder.Decode(out)
	if err != nil {
		log.Printf("Error decoding: %s", err)
		respondWithError(w, http.StatusInternalServerError, "Something went wrong")
		return
	}
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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(errDat)
}

func respondWithJSON(w http.ResponseWriter, code int, rawData any) {
	data, err := json.Marshal(rawData)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(data)
}
