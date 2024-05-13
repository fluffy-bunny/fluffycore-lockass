package purger

import (
	"context"
	"sync"
	"time"

	di "github.com/fluffy-bunny/fluffy-dozm-di"
	contracts_config "github.com/fluffy-bunny/fluffycore-lockaas/internal/contracts/config"
	contracts_lockclient "github.com/fluffy-bunny/fluffycore-lockaas/internal/contracts/lockclient"
	fluffycore_async "github.com/fluffy-bunny/fluffycore/async"
	async "github.com/reugn/async"
	zerolog "github.com/rs/zerolog"
	mongo_lock "github.com/square/mongo-lock"
)

type (
	service struct {
		config *contracts_config.Config

		lockClient contracts_lockclient.IMongoLockClient
		future     async.Future[fluffycore_async.AsyncResponse]
		mutex      sync.Mutex
		stop       chan struct{}
		ticker     *time.Ticker
	}
)

var stemService = (*service)(nil)

func init() {
	var _ contracts_lockclient.IPurger = (*service)(nil)
}
func (s *service) Ctor(config *contracts_config.Config,
	lockClient contracts_lockclient.IMongoLockClient) contracts_lockclient.IPurger {
	return &service{
		lockClient: lockClient,
		config:     config,
		stop:       make(chan struct{}),
		// add ticker
		ticker: time.NewTicker(time.Minute * 5),
	}
}
func AddSingletonPurger(cb di.ContainerBuilder) {
	di.AddSingleton[contracts_lockclient.IPurger](cb, stemService.Ctor)
}
func (s *service) Start(ctx context.Context) error {
	log := zerolog.Ctx(ctx).With().Logger()
	//--~--~--~--~-BARBED WIRE-~--~--~--
	s.mutex.Lock()
	defer s.mutex.Unlock()
	//--~--~--~--~-BARBED WIRE-~--~--~--

	if s.future != nil {
		return nil
	}
	log.Info().Msg("Purger starting")
	future := fluffycore_async.ExecuteWithPromiseAsync(func(promise async.Promise[fluffycore_async.AsyncResponse]) {
		var err error
		defer func() {
			log.Info().Msg("Purger stopped")
			promise.Success(&fluffycore_async.AsyncResponse{
				Message: "End Purger",
				Error:   err,
			})
		}()
		stopLoop := false
		for {
			select {
			case <-s.stop:
				stopLoop = true
			case <-s.ticker.C:
				ctx, cancel := context.WithTimeout(ctx, time.Second*30)
				defer cancel()
				err = s.lockClient.XLock(ctx, "lockaas-purger", "lockaas-purger",
					mongo_lock.LockDetails{
						TTL: 5 * 60,
					})
				if err != nil {
					continue
				}
				purger, err := s.lockClient.Purger(ctx)
				if err != nil {
					log.Error().Err(err).Msg("Purger")
					continue
				}
				_, err = purger.Purge(ctx)
				if err != nil {
					log.Error().Err(err).Msg("purger.Purge(ctx)")
					continue
				}
			}
			if stopLoop {
				break
			}
		}
	})
	s.future = future
	return nil
}

func (s *service) Stop(ctx context.Context) error {
	log := zerolog.Ctx(ctx).With().Logger()
	//--~--~--~--~-BARBED WIRE-~--~--~--
	s.mutex.Lock()
	defer s.mutex.Unlock()
	//--~--~--~--~-BARBED WIRE-~--~--~--
	log.Info().Msg("Purger stopping")
	defer func() {
		log.Info().Msg("Purger stopped")
	}()
	defer func() {
		s.future = nil
	}()
	if s.future == nil {
		return nil
	}

	close(s.stop)
	s.future.Join()

	return nil
}
