package main

import "go.mongodb.org/mongo-driver/bson/primitive"

type TestingConfig struct {
	Threads  int
	DocCount int
	Duration int
}

type TestingStrategy interface {
	runTestSequence(collection CollectionAPI, config TestingConfig)
	runTest(collection CollectionAPI, testType string, config TestingConfig, fetchDocIDs func(CollectionAPI) ([]primitive.ObjectID, error))
}
