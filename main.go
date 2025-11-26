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
	Timeout        int         `json:"int"`
	URL            string      `json:"url"`
	Heartbeat      int         `json:"heartbeat"`
	Strikes        int         `json:"strikes"`
	DiscordWebhook string      `json:"discord_webhook"`
}

var services []Service

var wasDown = false
var counter = 0

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
		checkAvailability(service.URL, service.Strikes, client, service.DiscordWebhook)
	}
}

func checkAvailability(url string, strikes int, client http.Client, discordWebhook string) bool {
	for i := range 3 {
		fmt.Println("try:", i+1)
		res, err := client.Get(url)
		if err != nil {
			fmt.Println(err)
			continue
		}
		if res.StatusCode >= 200 && res.StatusCode <= 299 {
			log.Println(url, "is up.")
			if wasDown {
				counter = 0
				wasDown = false
				notifyDiscord(fmt.Sprintf("✅ %v is back up! :)", url), discordWebhook)
			}
			return true
		}
	}

	counter++
	log.Printf("Website down - Strike %v/%v\n", counter, strikes)
	if discordWebhook != "" && counter == strikes {
		notifyDiscord(fmt.Sprintf("❌ Your service %v went down :(", url), discordWebhook)
		wasDown = true
		counter = 0
	}

	return false
}

func notifyDiscord(message string, discordWebhook string) {
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
