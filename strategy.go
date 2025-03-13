package main

import "go.mongodb.org/mongo-driver/bson/primitive"

type TestingConfig struct {
	Threads          int
	DocCount         int
	Duration         int
	LargeDocs        bool
	DropDb           bool
	OutputFilePrefix string
}

type TestingStrategy interface {
	runTestSequence(collection CollectionAPI, config TestingConfig)
	runTest(collection CollectionAPI, testType string, config TestingConfig, fetchDocIDs func(CollectionAPI, int64, string) ([]primitive.ObjectID, error))
}
