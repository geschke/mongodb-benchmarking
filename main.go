package main

import (
	"context"
	"flag"
	"log"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	var threads int
	var docCount int
	var uri string
	var testType string
	var outputFilePrefix string
	var duration int
	var runAll bool
	var largeDocs bool
	var dropDb bool
	var createIndex bool

	flag.IntVar(&threads, "threads", 10, "Number of threads for inserting, updating, upserting, or deleting documents")
	flag.IntVar(&docCount, "docs", 1000, "Total number of documents to insert, insertdoc, update, upsert, or delete")
	flag.StringVar(&uri, "uri", "mongodb://localhost:27017", "MongoDB URI")
	flag.StringVar(&testType, "type", "insert", "Test type: insert, update, upsert, delete, insertdoc or finddoc")
	flag.BoolVar(&runAll, "runAll", false, "Run all tests in order: insert, update, delete, upsert")
	flag.IntVar(&duration, "duration", 0, "Duration in seconds to run the test")
	flag.BoolVar(&largeDocs, "largeDocs", false, "Use large documents for testing")
	flag.BoolVar(&dropDb, "dropDb", true, "Drop the database before running the test")
	flag.StringVar(&outputFilePrefix, "out", "", "Output filename prefix (default: empty, using 'benchmark_results_*'")
	flag.BoolVar(&createIndex, "createIndex", false, "Create indexes before running insertdoc operation")

	flag.Parse()

	var strategy TestingStrategy
	var config TestingConfig

	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(uri).SetMaxPoolSize(uint64(threads)))
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer func(client *mongo.Client, ctx context.Context) {
		err := client.Disconnect(ctx)
		if err != nil {
			log.Fatalf("Failed to disconnect from MongoDB: %v", err)
		}
	}(client, context.Background())

	collection := client.Database("benchmarking").Collection("testdata")
	mongoCollection := &MongoDBCollection{Collection: collection}

	if duration > 0 {
		config = TestingConfig{
			Threads:          threads,
			Duration:         duration,
			DocCount:         docCount,
			LargeDocs:        largeDocs,
			DropDb:           dropDb,
			OutputFilePrefix: outputFilePrefix,
			CreateIndex:      createIndex,
		}
		strategy = DurationTestingStrategy{}
	} else {
		config = TestingConfig{
			Threads:          threads,
			DocCount:         docCount,
			LargeDocs:        largeDocs,
			DropDb:           dropDb,
			OutputFilePrefix: outputFilePrefix,
			CreateIndex:      createIndex,
		}
		strategy = DocCountTestingStrategy{}
	}
	if runAll || testType == "runAll" {
		strategy.runTestSequence(mongoCollection, config)
	} else if testType == "runDoc" {
		strategy.runTestSequenceDoc(mongoCollection, config)
	} else {
		strategy.runTest(mongoCollection, testType, config, fetchDocumentIDs)
	}
}
