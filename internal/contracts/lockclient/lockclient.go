package lockclient

import (
	"context"

	di "github.com/fluffy-bunny/fluffy-dozm-di"
	mongo_lock "github.com/square/mongo-lock"
)

type (
	IMongoLockClient interface {
		di.Disposable

		XLock(ctx context.Context, resourceName, lockId string, ld mongo_lock.LockDetails) error
		SLock(ctx context.Context, resourceName, lockId string, ld mongo_lock.LockDetails, maxConcurrent int) error
		Unlock(ctx context.Context, lockId string) ([]mongo_lock.LockStatus, error)
		Status(ctx context.Context, f mongo_lock.Filter) ([]mongo_lock.LockStatus, error)
		Renew(ctx context.Context, lockId string, ttl uint) ([]mongo_lock.LockStatus, error)
		Purger(ctx context.Context) (mongo_lock.Purger, error)
	}
	IPurger interface {
		Start(ctx context.Context) error
		Stop(ctx context.Context) error
	}
)
