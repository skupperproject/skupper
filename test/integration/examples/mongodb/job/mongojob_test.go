// +build job

package job

import (
	"context"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/skupperproject/skupper/pkg/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gotest.tools/assert"
)

type Post struct {
	Title string `json:"title,omitempty"`
	Body  string `json:"body,omitempty"`
}

var expected = Post{Title: "TheName", Body: "TheBody"}
var TOTAL_DB_DOCUMENTS = 1000
var DB_NAME = "my_database"
var COLLECTION_NAME = "posts"

func TestMongoJob(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	getClient := func(uri string) *mongo.Client {
		client, err := mongo.NewClient(options.Client().ApplyURI(uri))
		assert.Assert(t, err)
		err = client.Connect(ctx)
		assert.Assert(t, err)
		return client
	}
	client_a := getClient("mongodb://mongo-a:27017")
	client_b := getClient("mongodb://mongo-b:27017")
	defer client_a.Disconnect(ctx)
	defer client_b.Disconnect(ctx)

	// needed in case of a retry
	err := DropAllPosts(client_a, ctx)
	assert.Assert(t, err)

	err = InsertAllPosts(client_a, ctx)
	assert.Assert(t, err)

	err = CountDocuments(client_b, TOTAL_DB_DOCUMENTS, ctx)
	assert.Assert(t, err)

	err = PopAllExpectedPosts(client_a, ctx)
	assert.Assert(t, err)

	err = CountDocuments(client_b, 0, ctx)
	assert.Assert(t, err)
}

func InsertAllPosts(client *mongo.Client, ctx context.Context) error {
	post := Post{expected.Title, expected.Body}
	collection := client.Database(DB_NAME).Collection(COLLECTION_NAME)

	for i := 0; i < TOTAL_DB_DOCUMENTS; i++ {
		_, err := collection.InsertOne(ctx, post)
		if err != nil {
			log.Printf("error inserting document %d/%d", i+1, TOTAL_DB_DOCUMENTS)
			return err
		}
	}
	return nil
}

func CountDocuments(client *mongo.Client, expected_count int, ctx context.Context) error {
	var count int64
	// Need to retry in case data is not yet fully replicated
	err := utils.RetryWithContext(ctx, time.Second, func() (bool, error) {
		var err error
		collection := client.Database(DB_NAME).Collection(COLLECTION_NAME)
		count, err = collection.CountDocuments(ctx, bson.D{})
		if err != nil {
			return true, err
		}
		if int(count) == expected_count {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return err
	}
	if int(count) != expected_count {
		return fmt.Errorf("expected: %d, got %d", expected_count, count)
	}
	return nil
}

func PopAllExpectedPosts(client *mongo.Client, ctx context.Context) error {

	collection := client.Database(DB_NAME).Collection(COLLECTION_NAME)
	filter := bson.D{}
	var post Post

	count, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		return err
	}

	if int(count) != TOTAL_DB_DOCUMENTS {
		return fmt.Errorf("wrong number of documents in secondary DB, expected: %d, got: %d", TOTAL_DB_DOCUMENTS, count)

	}

	for i := 0; i < int(count); i++ {
		post = Post{}
		err = collection.FindOneAndDelete(ctx, filter).Decode(&post)
		if err != nil {
			log.Printf("error deleting document %d/%d", i+1, count)
			return err
		}
		if post != expected {
			return fmt.Errorf("extracted document different than expected. got: %v, expected: %v", post, expected)
		}

	}
	return nil
}

func DropAllPosts(client *mongo.Client, ctx context.Context) error {
	collection := client.Database(DB_NAME).Collection(COLLECTION_NAME)
	err := collection.Drop(ctx)
	if err != nil {
		log.Println("error dropping collection:", err)
		return err
	}

	return nil
}
