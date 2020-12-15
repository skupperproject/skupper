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

func InsertPost(client *mongo.Client, title string, body string) error {
	post := Post{title, body}
	collection := client.Database("my_database").Collection("posts")
	insertResult, err := collection.InsertOne(context.TODO(), post)
	if err != nil {
		return err
	}
	fmt.Println("Inserted post with ID:", insertResult.InsertedID)
	return nil
}

func GetPost(client *mongo.Client) (error, Post) {

	collection := client.Database("my_database").Collection("posts")
	filter := bson.D{}
	var post Post

	err := collection.FindOne(context.TODO(), filter).Decode(&post)
	if err != nil {
		return err, Post{}
	}
	return nil, post
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

	expected := Post{Title: "TheName", Body: "TheBody"}

	err := InsertPost(client_a, expected.Title, expected.Body)
	assert.Assert(t, err)

	err, post := GetPost(client_b)
	assert.Assert(t, err)
	assert.Assert(t, post == expected)
}
