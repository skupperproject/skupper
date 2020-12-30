// +build integration

package mongodb

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/k8s"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gotest.tools/assert"
)

func TestMain(m *testing.M) {
	base.ParseFlags()
	os.Exit(m.Run())
}

func TestMongo(t *testing.T) {
	needs := base.ClusterNeeds{
		NamespaceId:     "mongo",
		PublicClusters:  1,
		PrivateClusters: 1,
	}
	testRunner := &base.ClusterTestRunnerBase{}
	testRunner.BuildOrSkip(t, needs, nil)
	//ctx, cancel := context.WithCancel(context.Background())
	//base.HandleInterruptSignal(t, func(t *testing.T) {
	//base.TearDownSimplePublicAndPrivate(&testRunner.ClusterTestRunnerBase)
	//cancel()
	//})
	Run(context.Background(), t, testRunner)
}

type Post struct {
	Title string `json:"title,omitempty"`
	Body  string `json:"body,omitempty"`
}

var expected = Post{Title: "TheName", Body: "TheBody"}
var TOTAL_DB_DOCUMENTS = 1000
var DB_NAME = "my_database"
var COLLECTION_NAME = "posts"

func InsertAllPosts(client *mongo.Client, ctx context.Context) error {
	post := Post{expected.Title, expected.Body}
	collection := client.Database(DB_NAME).Collection(COLLECTION_NAME)

	for i := 0; i < TOTAL_DB_DOCUMENTS; i++ {
		_, err := collection.InsertOne(ctx, post)
		if err != nil {
			return err
		}
	}
	return nil
}

func CountDocuments(client *mongo.Client, expected_count int, ctx context.Context) error {
	collection := client.Database(DB_NAME).Collection(COLLECTION_NAME)
	count, err := collection.CountDocuments(ctx, bson.D{})
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
			return err
		}
		if post != expected {
			return fmt.Errorf("extracted document different than expected. got: %v, expected: %v", post, expected)
		}

	}
	return nil
}

func TestMongoJob(t *testing.T) {
	k8s.SkipTestJobIfMustBeSkipped(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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

	err := InsertAllPosts(client_a, ctx)
	assert.Assert(t, err)

	err = CountDocuments(client_b, TOTAL_DB_DOCUMENTS, ctx)
	assert.Assert(t, err)

	err = PopAllExpectedPosts(client_a, ctx)
	assert.Assert(t, err)

	err = CountDocuments(client_b, 0, ctx)
	assert.Assert(t, err)
}
