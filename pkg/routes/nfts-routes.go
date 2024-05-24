package nftroutes

import (
	nftcontroller "github.com/aman/nft-tracker/pkg/controllers"
	"github.com/gorilla/mux"
)

var NftDetails = func(router *mux.Router) {
	router.HandleFunc("/nft", nftcontroller.GetAllNfts)
	router.HandleFunc("/nft/{walletAddress}", nftcontroller.GetWalletNfts)
}
