package nftModel

import (
	"context"
	"errors"
	"log"
	"math/big"
	"os"
	"time"

	"github.com/aman/nft-tracker/pkg/config"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var collection *mongo.Collection

type NFT struct {
	ID              primitive.ObjectID `bson:"_id,omitempty"`
	NftID           int                `bson:"nftId,unique"`
	OwnerAddress    string             `bson:"ownerAddress"`
	ContractAddress string             `bson:"contractAddress"`
	TokenUri        string             `bson:"tokenUri"`
	TxHash          string             `bson:"txHash,unique"`
	TimeStamp       time.Time          `bson:"timestamp"`
}

func GetNftCollection() *mongo.Collection {
	collection = config.GetCollection(os.Getenv("DB_NAME"), "NFT")
	return collection
}

func CreateIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	indexModel := mongo.IndexModel{
		Keys:    bson.M{"nftId": 1},
		Options: options.Index().SetUnique(true),
	}

	_, err := collection.Indexes().CreateOne(ctx, indexModel)
	if err != nil {
		log.Fatalf("Failed to create index: %v", err)
	}

	log.Println("Unique index created on nftId")
}

func (nft *NFT) CreateUpdateNFT() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{"nftId": nft.NftID}
	update := bson.M{
		"$set": bson.M{
			"ownerAddress":    nft.OwnerAddress,
			"contractAddress": nft.ContractAddress,
			"txHash":          nft.TxHash,
			"timeStamp":       nft.TimeStamp,
		},
		"$setOnInsert": bson.M{
			"nftId": nft.NftID,
		},
	}

	opts := options.Update().SetUpsert(true)
	_, err := collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		log.Printf("Failed to insert NFT data into MongoDB: %v", err)
		return err
	}
	return nil
}

func GetAllNfts() ([]NFT, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	findOptions := options.Find()
	findOptions.SetSort(bson.D{{"nftId", -1}})

	cursor, err := collection.Find(ctx, bson.M{}, findOptions)
	if err != nil {
		log.Printf("Failed to find documents: %v", err)
		return nil, err
	}
	defer cursor.Close(ctx)

	var Nfts []NFT
	for cursor.Next(ctx) {
		var nft NFT
		if err := cursor.Decode(&nft); err != nil {
			log.Printf("Failed to decode document: %v", err)
			return nil, err
		}
		Nfts = append(Nfts, nft)
	}
	if err := cursor.Err(); err != nil {
		log.Printf("Cursor error: %v", err)
		return nil, err
	}

	return Nfts, nil
}

func GetWalletNfts(walletAddress string) ([]NFT, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	findOptions := options.Find()
	findOptions.SetSort(bson.D{{"nftId", -1}})

	cursor, err := collection.Find(ctx, bson.M{"ownerAddress": walletAddress}, findOptions)
	if err != nil {
		log.Printf("Failed to find documents: %v", err)
		return nil, err
	}
	defer cursor.Close(ctx)

	var Nfts []NFT
	for cursor.Next(ctx) {
		var nft NFT
		if err := cursor.Decode(&nft); err != nil {
			log.Printf("Failed to decode document: %v", err)
			return nil, err
		}

		Nfts = append(Nfts, nft)
	}

	if err := cursor.Err(); err != nil {
		log.Printf("Cursor error: %v", err)
		return nil, err
	}

	return Nfts, nil
}

// Helper function to convert big.Int to int
func BigIntToInt(b *big.Int) (int, error) {
	if b.IsInt64() {
		i64 := b.Int64()
		if i64 >= int64(^int(0)) || i64 <= int64(-1<<63) {
			return int(i64), nil
		}
	}
	return 0, errors.New("big.Int value is out of int range")
}
