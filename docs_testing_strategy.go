package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/rcrowley/go-metrics"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type DocCountTestingStrategy struct{}

func (t DocCountTestingStrategy) runTestSequence(collection CollectionAPI, config TestingConfig) {
	tests := []string{"insert", "update", "delete", "upsert", "insertdoc"}
	for _, test := range tests {
		t.runTest(collection, test, config, fetchDocumentIDs)
	}
}

func (t DocCountTestingStrategy) runTest(collection CollectionAPI, testType string, config TestingConfig, fetchDocIDs func(CollectionAPI, int64, string) ([]primitive.ObjectID, error)) {
	if testType == "insert" || testType == "upsert" || testType == "insertdoc" {
		if config.DropDb {
			if err := collection.Drop(context.Background()); err != nil {
				log.Fatalf("Failed to drop collection: %v", err)
			}
			log.Println("Collection dropped. Starting new rate test...")
		} else {
			log.Println("Collection stays. Dropping disabled.")
		}
	} else {
		log.Printf("Starting %s test...\n", testType)
	}

	// Create indexes before insertdoc test begins
	if testType == "insertdoc" && config.CreateIndex == true {
		log.Println("Creating indexes for insertdoc benchmark...")

		indexes := []mongo.IndexModel{
			{Keys: bson.D{{Key: "author", Value: 1}}},
			{Keys: bson.D{{Key: "tags", Value: 1}}},
			{Keys: bson.D{{Key: "timestamp", Value: -1}}},
			{Keys: bson.D{{Key: "content", Value: "text"}}},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		mongoColl, ok := collection.(*MongoDBCollection)
		if !ok {
			log.Println("Index creation skipped: Collection is not a MongoDBCollection")
		} else {
			_, err := mongoColl.Indexes().CreateMany(ctx, indexes)
			if err != nil {
				log.Printf("Failed to create indexes: %v", err)
			} else {
				log.Println("Indexes created successfully.")
			}
		}
	}

	var partitions [][]primitive.ObjectID

	var threads = config.Threads
	var docCount = config.DocCount

	// Prepare partitions based on test type
	switch testType {
	case "delete":
		// Fetch document IDs as ObjectId and partition them
		docIDs, err := fetchDocIDs(collection, int64(config.DocCount), testType)
		if err != nil {
			log.Fatalf("Failed to fetch document IDs: %v", err)
		}
		partitions = make([][]primitive.ObjectID, threads)
		for i, id := range docIDs {
			partitions[i%threads] = append(partitions[i%threads], id)
		}

	case "insert", "upsert", "insertdoc":
		partitions = make([][]primitive.ObjectID, threads)
		for i := 0; i < docCount; i++ {
			partitions[i%threads] = append(partitions[i%threads], primitive.NewObjectID())
		}

	case "update":
		docIDs, err := fetchDocIDs(collection, int64(config.DocCount), testType)
		if err != nil {
			log.Fatalf("Failed to fetch document IDs: %v", err)
		}

		partitions = make([][]primitive.ObjectID, threads)
		for i := 0; i < len(docIDs); i++ {
			docID := docIDs[rand.Intn(len(docIDs))]
			partitions[i%threads] = append(partitions[i%threads], docID)
		}
	default:
		log.Fatalf("Unknown or unsupported test type, exiting...")
	}

	// Start the ticker just before starting the main workload goroutines
	insertRate := metrics.NewMeter()
	var records [][]string
	records = append(records, []string{"t", "count", "mean", "m1_rate", "m5_rate", "m15_rate", "mean_rate"})

	var doc interface{}
	generator := NewDocumentGenerator()
	/*var data = make([]byte, 1024*2)
	for i := 0; i < len(data); i++ {
		data[i] = byte(rand.Intn(256))
	}*/

	secondTicker := time.NewTicker(1 * time.Second)
	defer secondTicker.Stop()
	go func() {
		for range secondTicker.C {
			timestamp := time.Now().Unix()
			count := insertRate.Count()
			mean := insertRate.RateMean()
			m1Rate := insertRate.Rate1()
			m5Rate := insertRate.Rate5()
			m15Rate := insertRate.Rate15()

			log.Printf("Timestamp: %d, Document Count: %d, Mean Rate: %.2f docs/sec, m1_rate: %.2f, m5_rate: %.2f, m15_rate: %.2f",
				timestamp, count, mean, m1Rate, m5Rate, m15Rate)

			record := []string{
				fmt.Sprintf("%d", timestamp),
				fmt.Sprintf("%d", count),
				fmt.Sprintf("%.6f", mean),
				fmt.Sprintf("%.6f", m1Rate),
				fmt.Sprintf("%.6f", m5Rate),
				fmt.Sprintf("%.6f", m15Rate),
			}
			records = append(records, record)
		}
	}()

	// Launch threads based on the specific workload type
	var wg sync.WaitGroup
	wg.Add(threads)

	for i := 0; i < threads; i++ {
		go func(partition []primitive.ObjectID) {
			defer wg.Done()
			for _, docID := range partition {
				switch testType {
				case "insert":
					if config.LargeDocs {
						//doc = bson.M{"threadRunCount": i, "rnd": rand.Int63(), "v": 1, "data": data}
						doc = generator.GenerateLarge(i)
					} else {
						//doc = bson.M{"threadRunCount": i, "rnd": rand.Int63(), "v": 1}
						doc = generator.GenerateSimple(i)
					}
					_, err := collection.InsertOne(context.Background(), doc)
					if err == nil {
						insertRate.Mark(1)
					} else {
						log.Printf("Insert failed: %v", err)
					}
				case "insertdoc":
					doc = generator.GenerateComplex(i)
					_, err := collection.InsertOne(context.Background(), doc)
					if err == nil {
						insertRate.Mark(1)
					} else {
						log.Printf("Insertdoc failed: %v", err)
					}
				case "update":
					randomDocID := partition[rand.Intn(len(partition))]
					filter := bson.M{"_id": randomDocID}
					update := bson.M{"$set": bson.M{"updatedAt": time.Now().Unix(), "rnd": rand.Int63()}}
					_, err := collection.UpdateOne(context.Background(), filter, update)
					if err == nil {
						insertRate.Mark(1)
					} else {
						log.Printf("Update failed for _id %v: %v", docID, err)
					}

				case "upsert":
					randomDocID := partition[rand.Intn(len(partition)/2)]
					filter := bson.M{"_id": randomDocID}
					update := bson.M{"$set": bson.M{"updatedAt": time.Now().Unix(), "rnd": rand.Int63()}}
					opts := options.Update().SetUpsert(true)
					_, err := collection.UpdateOne(context.Background(), filter, update, opts)
					if err == nil {
						insertRate.Mark(1)
					} else {
						log.Printf("Upsert failed for _id %v: %v", docID, err)
					}

				case "delete":
					// Use ObjectId in the filter for delete
					filter := bson.M{"_id": docID}
					result, err := collection.DeleteOne(context.Background(), filter)
					if err != nil {
						log.Printf("Delete failed for _id %v: %v", docID, err)
						continue // Move to next document without retrying
					}
					if result.DeletedCount > 0 {
						insertRate.Mark(1)
					}
				}
			}
		}(partitions[i])
	}

	wg.Wait()

	// Final metrics recording
	timestamp := time.Now().Unix()
	count := insertRate.Count()
	mean := insertRate.RateMean()
	m1Rate := insertRate.Rate1()
	m5Rate := insertRate.Rate5()
	m15Rate := insertRate.Rate15()

	finalRecord := []string{
		fmt.Sprintf("%d", timestamp),
		fmt.Sprintf("%d", count),
		fmt.Sprintf("%.6f", mean),
		fmt.Sprintf("%.6f", m1Rate),
		fmt.Sprintf("%.6f", m5Rate),
		fmt.Sprintf("%.6f", m15Rate),
	}
	records = append(records, finalRecord)

	filenamePrefix := "benchmark_results"
	if config.OutputFilePrefix != "" {
		filenamePrefix = config.OutputFilePrefix
	}

	filename := fmt.Sprintf("%s_%s.csv", filenamePrefix, testType)
	file, err := os.Create(filename)
	if err != nil {
		log.Fatalf("Failed to create CSV file: %v", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	if err := writer.WriteAll(records); err != nil {
		log.Fatalf("Failed to write records to CSV: %v", err)
	}
	writer.Flush()

	fmt.Printf("Benchmarking completed. Results saved to %s\n", filename)
}
