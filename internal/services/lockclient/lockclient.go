package lockclient

import (
	"context"
	"sync"
	"time"

	di "github.com/fluffy-bunny/fluffy-dozm-di"
	contracts_config "github.com/fluffy-bunny/fluffycore-lockaas/internal/contracts/config"
	contracts_mongo "github.com/fluffy-bunny/fluffycore-lockaas/internal/contracts/lockclient"
	mongo_lock "github.com/square/mongo-lock"
	driver_mongo "go.mongodb.org/mongo-driver/mongo"
	options "go.mongodb.org/mongo-driver/mongo/options"
	mongo_writeconcern "go.mongodb.org/mongo-driver/mongo/writeconcern"
)

type (
	service struct {
		config     *contracts_config.Config
		client     *driver_mongo.Client
		collection *driver_mongo.Collection
		lockClient *mongo_lock.Client
		lock       sync.Mutex
	}
)

func init() {
	var _ contracts_mongo.IMongoLockClient = (*service)(nil)
}

func AddSingletonLockClient(cb di.ContainerBuilder) {
	di.AddSingleton[contracts_mongo.IMongoLockClient](cb,
		func(config *contracts_config.Config) contracts_mongo.IMongoLockClient {
			return &service{
				config: config,
			}
		})
}

func (s *service) Collection(ctx context.Context) (*driver_mongo.Collection, error) {
	//--~--~--~--~--~--~--~--~--~--~--~--~--~--~--~--~--~--~--~--~--
	s.lock.Lock()
	defer s.lock.Unlock()
	//--~--~--~--~--~--~--~--~--~--~--~--~--~--~--~--~--~--~--~--~--
	if s.client == nil {
		ctx, cancel := context.WithTimeout(ctx, time.Second*30)
		defer cancel()

		wc := &mongo_writeconcern.WriteConcern{
			W: "majority",
		}
		client, err := driver_mongo.Connect(ctx, options.Client().
			ApplyURI(s.config.MongoConfig.MongoUrl).
			SetWriteConcern(wc))
		if err != nil {
			return nil, err
		}
		s.client = client
		col := s.client.Database(s.config.MongoConfig.Database).Collection("fluffycore_lockaas")
		s.collection = col
	}

	return s.collection, nil
}
func (s *service) LockClient(ctx context.Context) (*mongo_lock.Client, error) {
	doGetLockClient := func() *mongo_lock.Client {
		//--~--~--~--~--~--~--~--~--~--~--~--~--~--~--~--~--~--~--~--~--
		s.lock.Lock()
		defer s.lock.Unlock()
		//--~--~--~--~--~--~--~--~--~--~--~--~--~--~--~--~--~--~--~--~--
		if s.lockClient != nil {
			return s.lockClient
		}
		return nil
	}
	if client := doGetLockClient(); client != nil {
		return client, nil
	}
	collection, err := s.Collection(ctx)
	if err != nil {
		return nil, err
	}
	//--~--~--~--~--~--~--~--~--~--~--~--~--~--~--~--~--~--~--~--~--
	s.lock.Lock()
	defer s.lock.Unlock()
	//--~--~--~--~--~--~--~--~--~--~--~--~--~--~--~--~--~--~--~--~--
	lockClient := mongo_lock.NewClient(collection)
	s.lockClient = lockClient
	return lockClient, nil
}

func (s *service) Dispose() {
	if s.client != nil {
		s.client.Disconnect(context.Background())
	}
}
func (s *service) XLock(ctx context.Context, resourceName, lockId string, ld mongo_lock.LockDetails) error {
	lockClient, err := s.LockClient(ctx)
	if err != nil {
		return err
	}
	return lockClient.XLock(ctx, resourceName, lockId, ld)
}
func (s *service) SLock(ctx context.Context, resourceName, lockId string, ld mongo_lock.LockDetails, maxConcurrent int) error {
	lockClient, err := s.LockClient(ctx)
	if err != nil {
		return err
	}
	return lockClient.SLock(ctx, resourceName, lockId, ld, maxConcurrent)
}
func (s *service) Unlock(ctx context.Context, lockId string) ([]mongo_lock.LockStatus, error) {
	lockClient, err := s.LockClient(ctx)
	if err != nil {
		return nil, err
	}
	return lockClient.Unlock(ctx, lockId)
}
func (s *service) Status(ctx context.Context, f mongo_lock.Filter) ([]mongo_lock.LockStatus, error) {
	lockClient, err := s.LockClient(ctx)
	if err != nil {
		return nil, err
	}
	return lockClient.Status(ctx, f)
}
func (s *service) Renew(ctx context.Context, lockId string, ttl uint) ([]mongo_lock.LockStatus, error) {
	lockClient, err := s.LockClient(ctx)
	if err != nil {
		return nil, err
	}
	return lockClient.Renew(ctx, lockId, ttl)
}
