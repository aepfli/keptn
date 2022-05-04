package db

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	keptnmongoutils "github.com/keptn/go-utils/pkg/common/mongoutils"

	logger "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
)

var databaseName string

var mutex = &sync.Mutex{}

const clientCreationFailed = "failed to create mongo client: %v"
const clientConnectionFailed = "failed to connect client to MongoDB: %v"

// MongoDBConnection takes care of establishing a connection to the mongodb
type MongoDBConnection struct {
	Client *mongo.Client
}

func getMongoDBWriteConcernTimeout() time.Duration {
	timeoutString := os.Getenv("MONGODB_WRITECONCERN_TIMEOUT")
	timeout, err := strconv.Atoi(timeoutString)
	if err != nil {
		logger.Errorf("failed to read MongoDB WriteConcern Timeout from env variable: %v", err.Error())
		return 30 * time.Second
	}
	return time.Duration(timeout) * time.Second
}

// EnsureDBConnection makes sure a connection to the mongodb is established
func (m *MongoDBConnection) EnsureDBConnection() error {
	// Mutex is neccessary for not creating multiple clients after restarting the pod
	mutex.Lock()
	defer mutex.Unlock()
	var err error
	// attention: not calling the cancel() function likely causes memory leaks
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if m.Client == nil {
		logger.Info("No MongoDB client has been initialized yet. Creating a new one.")
		return m.connectMongoDBClient()
	} else if err = m.Client.Ping(ctx, nil); err != nil {
		logger.Info("MongoDB client lost connection. Attempting reconnect.")
		ctxDisconnect, cancelDisconnect := context.WithTimeout(context.TODO(), 30*time.Second)
		defer cancelDisconnect()
		err2 := m.Client.Disconnect(ctxDisconnect)
		if err2 != nil {
			logger.Errorf("failed to disconnect client from MongoDB: %v", err2)
		}
		m.Client = nil
		return m.connectMongoDBClient()
	}
	return nil
}

func (m *MongoDBConnection) connectMongoDBClient() error {
	var err error
	connectionString, dbName, err := keptnmongoutils.GetMongoConnectionStringFromEnv()
	if err != nil {
		logger.Errorf(clientCreationFailed, err)
		return fmt.Errorf(clientCreationFailed, err)
	}
	databaseName = dbName
	clientOptions := options.Client().SetWriteConcern(writeconcern.New(writeconcern.WMajority(), writeconcern.WTimeout(getMongoDBWriteConcernTimeout())))
	clientOptions = clientOptions.ApplyURI(connectionString)
	clientOptions = clientOptions.SetConnectTimeout(30 * time.Second)
	m.Client, err = mongo.NewClient(clientOptions)
	if err != nil {
		logger.Errorf(clientCreationFailed, err)
		return fmt.Errorf(clientCreationFailed, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = m.Client.Connect(ctx)
	if err != nil {
		logger.Errorf(clientConnectionFailed, err)
		return fmt.Errorf(clientConnectionFailed, err)
	}
	if err = m.Client.Ping(ctx, nil); err != nil {
		logger.Errorf(clientConnectionFailed, err)
		return fmt.Errorf(clientConnectionFailed, err)
	}

	logger.Info("Successfully connected to MongoDB")
	return nil
}
