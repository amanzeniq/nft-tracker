package main

import (
	"context"
	"log"
	"net/http"

	"github.com/aman/nft-tracker/pkg/config"
	nftroutes "github.com/aman/nft-tracker/pkg/routes"
	trackingService "github.com/aman/nft-tracker/pkg/services"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	config.ConnectDB()

	tracker, err := trackingService.NewTransferEventTracker()
	if err != nil {
		log.Fatalf("Failed to initialize transfer event tracker: %v", err)
	}

	go func() {
		err := tracker.TrackTransferEvents(context.Background())
		if err != nil {
			log.Fatalf("Failed to track events: %v", err)
		}
	}()

	r := mux.NewRouter()
	nftroutes.NftDetails(r)
	http.Handle("/", r)
	log.Fatal(http.ListenAndServe("localhost:3000", r))
}
