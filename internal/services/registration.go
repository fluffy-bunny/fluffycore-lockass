package services

import (
	di "github.com/fluffy-bunny/fluffy-dozm-di"
)

// put all services you want shared between the echo and grpc servers here
// NOTE: they are NOT the same instance, but they are the same type in context of the server.
func ConfigureServices(builder di.ContainerBuilder) {
}
