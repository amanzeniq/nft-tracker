package trackingService

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"os"
	"strconv"
	"strings"
	"time"

	nftModel "github.com/aman/nft-tracker/pkg/models"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"go.mongodb.org/mongo-driver/mongo"
)

type TransferEventTracker struct {
	client        *ethclient.Client
	collection    *mongo.Collection
	contractAddrs []common.Address
}

func NewTransferEventTracker() (*TransferEventTracker, error) {
	collection := nftModel.GetNftCollection()

	if collection == nil {
		log.Fatal("Failed to get MongoDB collection")
	}

	nftModel.CreateIndexes()

	rpcEndpoint := os.Getenv("ETH_RPC_ENDPOINT")
	if rpcEndpoint == "" {
		return nil, errors.New("ETH_RPC_ENDPOINT environment variable is not set")
	}
	client, err := ethclient.Dial(rpcEndpoint)
	if err != nil {
		return nil, fmt.Errorf("error connecting to Ethereum client: %v", err)
	}

	contractAddrsEnv := os.Getenv("CONTRACT_ADDRESSES")
	if contractAddrsEnv == "" {
		return nil, errors.New("CONTRACT_ADDRESSES environment variable is not set")
	}

	var addrStrings []string
	err = json.Unmarshal([]byte(contractAddrsEnv), &addrStrings)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CONTRACT_ADDRESSES environment variable: %v", err)
	}

	contractAddrs := make([]common.Address, 0, len(addrStrings))
	for _, addr := range addrStrings {
		parsedAddr := common.HexToAddress(addr)
		if parsedAddr == (common.Address{}) {
			log.Printf("Invalid contract address: %s", addr)
			continue
		}
		contractAddrs = append(contractAddrs, parsedAddr)
	}

	if len(contractAddrs) == 0 {
		return nil, errors.New("no valid contract addresses found in CONTRACT_ADDRESSES environment variable")
	}

	return &TransferEventTracker{
		client:        client,
		collection:    collection,
		contractAddrs: contractAddrs,
	}, nil
}

func (t *TransferEventTracker) TrackTransferEvents(ctx context.Context) error {
	transferEventSignature := []byte("Transfer(address,address,uint256)")
	transferEventHash := crypto.Keccak256Hash(transferEventSignature)

	fromBlockStr := os.Getenv("FROM_BLOCK")
	if fromBlockStr == "" {
		return errors.New("FROM_BLOCK environment variable is not set")
	}
	fromBlockInt, err := strconv.ParseInt(fromBlockStr, 10, 64)
	if err != nil {
		return fmt.Errorf("failed to parse FROM_BLOCK environment variable: %v", err)
	}
	startBlock := big.NewInt(fromBlockInt)

	header, err := t.client.HeaderByNumber(ctx, nil)
	if err != nil {
		log.Printf("Failed to get latest block header: %v\n", err)
		return err
	}
	latestBlock := header.Number

	query := ethereum.FilterQuery{
		FromBlock: startBlock,
		ToBlock:   latestBlock,
		Addresses: t.contractAddrs,
		Topics:    [][]common.Hash{{transferEventHash}},
	}

	historicalLogs, err := t.client.FilterLogs(ctx, query)
	if err != nil {
		log.Printf("Failed to fetch historical Transfer events: %v\n", err)
		return err
	}

	for _, delog := range historicalLogs {
		err = t.processTransferLog(ctx, delog)
		if err != nil {
			log.Printf("Failed to process historical Transfer event log: %v\n", err)
		}
	}

	interval := os.Getenv("FETCH_INTERVAL")
	if interval == "" {
		interval = "10m"
	}
	duration, err := time.ParseDuration(interval)
	if err != nil {
		log.Printf("Failed to parse FETCH_INTERVAL: %v, defaulting to 10 minutes\n", err)
		duration = 10 * time.Minute
	}

	ticker := time.NewTicker(duration)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			t.fetchNewLogs(ctx, transferEventHash, latestBlock)
		case <-ctx.Done():
			log.Printf("Context done, stopping event tracking")
			return ctx.Err()
		}
	}
	return nil
}

func (t *TransferEventTracker) fetchNewLogs(ctx context.Context, transferEventHash common.Hash, fromBlock *big.Int) {
	header, err := t.client.HeaderByNumber(ctx, nil)
	if err != nil {
		log.Printf("Failed to get latest block header: %v\n", err)
		return
	}
	latestBlock := header.Number

	query := ethereum.FilterQuery{
		FromBlock: fromBlock,
		ToBlock:   latestBlock,
		Addresses: t.contractAddrs,
		Topics:    [][]common.Hash{{transferEventHash}},
	}

	logs, err := t.client.FilterLogs(ctx, query)
	if err != nil {
		log.Printf("Failed to fetch new Transfer events: %v\n", err)
		return
	}

	for _, delog := range logs {
		err = t.processTransferLog(ctx, delog)
		if err != nil {
			log.Printf("Failed to process new Transfer event log: %v\n", err)
		}
	}
}

func (t *TransferEventTracker) processTransferLog(ctx context.Context, delog types.Log) error {
	to, tokenId, err := decodeTransferLog(delog)
	if err != nil {
		log.Printf("Failed to decode Transfer event log: %v", err)
		return fmt.Errorf("failed to decode Transfer event log: %v", err)
	}

	tokenIDInt, err := nftModel.BigIntToInt(tokenId)
	if err != nil {
		log.Printf("Failed to convert tokenId to int: %v", err)
		return fmt.Errorf("failed to convert tokenId to int: %v", err)
	}

	log.Printf("Processing log for token ID: %s, to address: %s", tokenId.String(), to.Hex())

	nft := nftModel.NFT{
		NftID:           tokenIDInt,
		OwnerAddress:    to.Hex(),
		ContractAddress: delog.Address.Hex(),
		TxHash:          delog.TxHash.Hex(),
		TimeStamp:       time.Now(),
	}

	log.Printf("NFT object to insert: %+v", nft)

	err = nft.CreateUpdateNFT()
	if err != nil {
		log.Printf("Failed to create/update NFT: %v", err)
	}

	// filter := bson.M{"nftId": nft.NftID}
	// update := bson.M{
	// 	"$set": bson.M{
	// 		"ownerAddress":    nft.OwnerAddress,
	// 		"contractAddress": nft.ContractAddress,
	// 		"txHash":          nft.TxHash,
	// 		"timeStamp":       nft.TimeStamp,
	// 	},
	// 	"$setOnInsert": bson.M{
	// 		"nftId": nft.NftID,
	// 	},
	// }

	// opts := options.Update().SetUpsert(true)
	// _, err = t.collection.UpdateOne(ctx, filter, update, opts)
	// if err != nil {
	// 	log.Printf("Failed to insert NFT data into MongoDB: %v", err)
	// 	return fmt.Errorf("failed to insert NFT data into MongoDB: %v", err)
	// }

	log.Printf("Successfully inserted NFT data: %+v", nft)
	return nil
}

func decodeTransferLog(delog types.Log) (common.Address, *big.Int, error) {
	transferEventABI := `[
		{
			"anonymous": false,
			"inputs": [
				{
					"indexed": true,
					"internalType": "address",
					"name": "from",
					"type": "address"
				},
				{
					"indexed": true,
					"internalType": "address",
					"name": "to",
					"type": "address"
				},
				{
					"indexed": true,
					"internalType": "uint256",
					"name": "tokenId",
					"type": "uint256"
				}
			],
			"name": "Transfer",
			"type": "event"
		}
	]`
	contractABI, err := abi.JSON(strings.NewReader(transferEventABI))
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("failed to parse contract ABI: %v", err)
	}

	type LogTransfer struct {
		From    common.Address
		To      common.Address
		TokenId *big.Int
	}

	var transferEvent LogTransfer

	err = contractABI.UnpackIntoInterface(&transferEvent, "Transfer", delog.Data)
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("failed to unpack Transfer event log: %v", err)
	}

	transferEvent.From = common.HexToAddress(delog.Topics[1].Hex())
	transferEvent.To = common.HexToAddress(delog.Topics[2].Hex())
	transferEvent.TokenId = delog.Topics[3].Big()

	return transferEvent.To, transferEvent.TokenId, nil
}
