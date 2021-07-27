package db

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoClient struct {
	Client            *mongo.Client
	UserCollection    *mongo.Collection
	ReposCollecion    *mongo.Collection
	ArchiveCollection *mongo.Collection
}

func NewMongoClient() (*MongoClient, error) {
	// connect to mongodb
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		return nil, err
	}
	return &MongoClient{
		Client:            client,
		UserCollection:    client.Database("mydb").Collection("user"),
		ReposCollecion:    client.Database("mydb").Collection("repository"),
		ArchiveCollection: client.Database("mydb").Collection("imagehub"),
	}, nil
}
