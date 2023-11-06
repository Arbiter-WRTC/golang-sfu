package main

import (
	"io/ioutil"
	"log"
	"fmt"
	"crypto/tls"
	"os"
	"github.com/joho/godotenv"
	"net/http"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"arbiter-go-sfu/pkg/sfu"
	"github.com/pion/webrtc/v3"
	"encoding/json"
)

func main() {

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	cert, err := ioutil.ReadFile("server.crt")
	if err != nil {
		log.Fatalf("Failed to read certificate: %v", err)
	}

	key, err := ioutil.ReadFile("server.key")
	if err != nil {
		log.Fatalf("Failed to read key: %v", err)
	}

	tlsCert, err := tls.X509KeyPair(cert, key)
	if err != nil {
		log.Fatalf("Failed to create key pair: %v", err)
	}
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
	}

	rawRtcConfig := os.Getenv("RTC_CONFIG")
	if rawRtcConfig == "" {
		log.Fatal("RTC_CONFIG environment variable is not set or is empty")
	}

	var rtcConfig []webrtc.ICEServer

	// Unmarshal the JSON into the iceServers variable.
	jsonErr := json.Unmarshal([]byte(rawRtcConfig), &rtcConfig)
	if jsonErr != nil {
		log.Fatalf("Failed to unmarshal RTC_CONFIG: %v", jsonErr)
	}
	
	signalServerURL := os.Getenv("SIGNAL_SERVER_URL")
	if signalServerURL == "" {
		log.Fatal("SIGNAL_SERVER_URL environment variable is not set or is empty")
	}
	
	sfuId := os.Getenv("SFU_ID")
	if sfuId == "" {
		log.Fatal("SFU_ID environment variable is not set or is empty")
	}

	sfu := SFU.NewSFU(signalServerURL, sfuId, rtcConfig)
	fmt.Println(sfu)
	router := mux.NewRouter()
	handler := cors.Default().Handler(router)

	server := &http.Server{
		Addr:      ":443", 
		Handler:   handler,
		TLSConfig: tlsConfig,
	}

	log.Println("Starting HTTPS server on port 443")

	err = server.ListenAndServeTLS("", "")
	if err != nil {
			log.Fatalf("Failed to start server: %v", err)
	}
}