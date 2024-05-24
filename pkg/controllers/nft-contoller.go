package nftcontroller

import (
	"encoding/json"
	"log"
	"net/http"

	nftModel "github.com/aman/nft-tracker/pkg/models"
	"github.com/gorilla/mux"
)

func GetAllNfts(w http.ResponseWriter, r *http.Request) {
	nfts, err := nftModel.GetAllNfts()
	if err != nil {
		log.Printf("Error in fecthing nfts: %v", err)
	}

	w.Header().Set("Content-Type", "pkglication/json")
	w.WriteHeader(http.StatusOK)

	err = json.NewEncoder(w).Encode(nfts)
	if err != nil {
		log.Printf("Error encoding nfts: %v", err)
		http.Error(w, "Error encoding NFTs", http.StatusInternalServerError)
	}
}

func GetWalletNfts(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	walletAddress := vars["walletAddress"]

	nfts, err := nftModel.GetWalletNfts(walletAddress)
	if err != nil {
		log.Printf("Error in fetching nfts: %v", err)
	}

	w.Header().Set("Content-Type", "pkglication/json")
	w.WriteHeader(http.StatusOK)

	err = json.NewEncoder(w).Encode(nfts)
	if err != nil {
		log.Printf("Error encoding nfts: %v", err)
		http.Error(w, "Error encoding NFTs", http.StatusInternalServerError)
	}
}
