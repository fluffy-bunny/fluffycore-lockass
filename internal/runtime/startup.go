package runtime

import (
	"context"
	"encoding/json"

	di "github.com/fluffy-bunny/fluffy-dozm-di"
	internal_auth "github.com/fluffy-bunny/fluffycore-lockaas/internal/auth"
	contracts_config "github.com/fluffy-bunny/fluffycore-lockaas/internal/contracts/config"
	contracts_lockclient "github.com/fluffy-bunny/fluffycore-lockaas/internal/contracts/lockclient"
	myechoserver "github.com/fluffy-bunny/fluffycore-lockaas/internal/myechoserver"
	services "github.com/fluffy-bunny/fluffycore-lockaas/internal/services"
	services_health "github.com/fluffy-bunny/fluffycore-lockaas/internal/services/health"
	services_lockaas "github.com/fluffy-bunny/fluffycore-lockaas/internal/services/lockaas"
	services_lockclient "github.com/fluffy-bunny/fluffycore-lockaas/internal/services/lockclient"
	services_purger "github.com/fluffy-bunny/fluffycore-lockaas/internal/services/purger"
	internal_version "github.com/fluffy-bunny/fluffycore-lockaas/internal/version"
	fluffycore_async "github.com/fluffy-bunny/fluffycore/async"
	fluffycore_contracts_ddprofiler "github.com/fluffy-bunny/fluffycore/contracts/ddprofiler"
	fluffycore_contracts_middleware "github.com/fluffy-bunny/fluffycore/contracts/middleware"
	fluffycore_contracts_middleware_auth_jwt "github.com/fluffy-bunny/fluffycore/contracts/middleware/auth/jwt"
	fluffycore_contracts_runtime "github.com/fluffy-bunny/fluffycore/contracts/runtime"
	core_echo_runtime "github.com/fluffy-bunny/fluffycore/echo/runtime"
	fluffycore_middleware_auth_jwt "github.com/fluffy-bunny/fluffycore/middleware/auth/jwt"
	fluffycore_middleware_claimsprincipal "github.com/fluffy-bunny/fluffycore/middleware/claimsprincipal"
	fluffycore_middleware_correlation "github.com/fluffy-bunny/fluffycore/middleware/correlation"
	fluffycore_middleware_dicontext "github.com/fluffy-bunny/fluffycore/middleware/dicontext"
	fluffycore_middleware_logging "github.com/fluffy-bunny/fluffycore/middleware/logging"
	core_runtime "github.com/fluffy-bunny/fluffycore/runtime"
	fluffycore_services_ddprofiler "github.com/fluffy-bunny/fluffycore/services/ddprofiler"
	fluffycore_utils_redact "github.com/fluffy-bunny/fluffycore/utils/redact"
	status "github.com/gogo/status"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	async "github.com/reugn/async"
	zerolog "github.com/rs/zerolog"
	log "github.com/rs/zerolog/log"
	codes "google.golang.org/grpc/codes"
)

type (
	startup struct {
		fluffycore_contracts_runtime.UnimplementedStartup
		RootContainer di.Container

		configOptions *fluffycore_contracts_runtime.ConfigOptions
		config        *contracts_config.Config

		ddProfiler          fluffycore_contracts_ddprofiler.IDataDogProfiler
		myEchoServerFuture  async.Future[fluffycore_async.AsyncResponse]
		myEchoServerRuntime *core_echo_runtime.Runtime
		purger              contracts_lockclient.IPurger
		cancel              context.CancelFunc
	}
)

func NewStartup() fluffycore_contracts_runtime.IStartup {
	return &startup{}
}
func (s *startup) SetRootContainer(container di.Container) {
	s.RootContainer = container
}
func (s *startup) GetConfigOptions() *fluffycore_contracts_runtime.ConfigOptions {
	log := log.With().Caller().Str("method", "GetConfigOptions").Logger()

	// here we load a config file and merge it over the default.
	initialConfigOptions := &fluffycore_contracts_runtime.ConfigOptions{
		Destination: &contracts_config.InitialConfig{},
		RootConfig:  contracts_config.ConfigDefaultJSON,
	}
	err := core_runtime.LoadConfig(initialConfigOptions)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}
	err = onLoadCoreConfig(context.Background(),
		initialConfigOptions.Destination.(*contracts_config.InitialConfig).ConfigFiles.CorePath)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to onLoadCoreConfig")
	}
	defaultConfig := &contracts_config.Config{}
	err = json.Unmarshal([]byte(contracts_config.ConfigDefaultJSON), defaultConfig)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to unmarshal ConfigDefaultJSON")
	}
	log.Info().Interface("defaultConfig", defaultConfig).Msg("config after merge")

	s.config = &contracts_config.Config{}
	s.configOptions = &fluffycore_contracts_runtime.ConfigOptions{
		Destination: s.config,
		RootConfig:  contracts_config.ConfigDefaultJSON,
	}
	return s.configOptions
}
func (s *startup) ConfigureServices(ctx context.Context, builder di.ContainerBuilder) {
	log := zerolog.Ctx(ctx).With().Str("method", "Configure").Logger()
	dst, err := fluffycore_utils_redact.CloneAndRedact(s.configOptions.Destination)
	if err != nil {
		panic(err)
	}
	log.Info().Interface("config", dst).Msg("config")
	config := s.configOptions.Destination.(*contracts_config.Config)
	config.DDProfilerConfig.ApplicationEnvironment = config.ApplicationEnvironment
	config.DDProfilerConfig.ServiceName = config.ApplicationName
	config.DDProfilerConfig.Version = internal_version.Version()
	di.AddInstance[*fluffycore_contracts_ddprofiler.Config](builder, config.DDProfilerConfig)
	di.AddInstance[*contracts_config.Config](builder, config)

	services.ConfigureServices(builder)
	fluffycore_services_ddprofiler.AddSingletonIProfiler(builder)
	services_health.AddHealthService(builder)
	services_lockaas.AddLockaasService(builder)
	services_purger.AddSingletonPurger(builder)
	services_lockclient.AddSingletonLockClient(builder)
	issuerConfigs := &fluffycore_contracts_middleware_auth_jwt.IssuerConfigs{}
	for idx := range s.config.JWTValidators.Issuers {
		issuerConfigs.IssuerConfigs = append(issuerConfigs.IssuerConfigs,
			&fluffycore_contracts_middleware_auth_jwt.IssuerConfig{
				OAuth2Config: &fluffycore_contracts_middleware_auth_jwt.OAuth2Config{
					Issuer:  s.config.JWTValidators.Issuers[idx],
					JWKSUrl: s.config.JWTValidators.JWKSURLS[idx],
				},
			})
	}
	fluffycore_middleware_auth_jwt.AddValidators(builder, issuerConfigs)
}
func (s *startup) Configure(ctx context.Context, rootContainer di.Container, unaryServerInterceptorBuilder fluffycore_contracts_middleware.IUnaryServerInterceptorBuilder, streamServerInterceptorBuilder fluffycore_contracts_middleware.IStreamServerInterceptorBuilder) {
	log := zerolog.Ctx(ctx).With().Str("method", "Configure").Logger()

	// puts a zerlog logger into the request context
	log.Info().Msg("adding unaryServerInterceptorBuilder: fluffycore_middleware_logging.EnsureContextLoggingUnaryServerInterceptor")
	unaryServerInterceptorBuilder.Use(fluffycore_middleware_logging.EnsureContextLoggingUnaryServerInterceptor())
	log.Info().Msg("adding streamServerInterceptorBuilder: fluffycore_middleware_logging.EnsureContextLoggingStreamServerInterceptor")
	streamServerInterceptorBuilder.Use(fluffycore_middleware_logging.EnsureContextLoggingStreamServerInterceptor())

	// log correlation and spans
	unaryServerInterceptorBuilder.Use(fluffycore_middleware_correlation.EnsureCorrelationIDUnaryServerInterceptor())
	// dicontext is responsible of create a scoped context for each request.
	log.Info().Msg("adding unaryServerInterceptorBuilder: fluffycore_middleware_dicontext.UnaryServerInterceptor")
	unaryServerInterceptorBuilder.Use(fluffycore_middleware_dicontext.UnaryServerInterceptor(rootContainer))
	log.Info().Msg("adding streamServerInterceptorBuilder: fluffycore_middleware_dicontext.StreamServerInterceptor")
	streamServerInterceptorBuilder.Use(fluffycore_middleware_dicontext.StreamServerInterceptor(rootContainer))

	// auth
	log.Info().Msg("adding unaryServerInterceptorBuilder: fluffycore_middleware_auth_jwt.UnaryServerInterceptor")
	unaryServerInterceptorBuilder.Use(fluffycore_middleware_auth_jwt.UnaryServerInterceptor(rootContainer))

	// Here the gating happens
	grpcEntrypointClaimsMap := internal_auth.BuildGrpcEntrypointPermissionsClaimsMap()
	// claims principal
	log.Info().Msg("adding unaryServerInterceptorBuilder: fluffycore_middleware_claimsprincipal.UnaryServerInterceptor")
	unaryServerInterceptorBuilder.Use(fluffycore_middleware_claimsprincipal.FinalAuthVerificationMiddlewareUsingClaimsMapWithZeroTrustV2(grpcEntrypointClaimsMap))

	// last is the recovery middleware
	customFunc := func(p interface{}) (err error) {
		return status.Errorf(codes.Unknown, "panic triggered: %v", p)
	}
	opts := []grpc_recovery.Option{
		grpc_recovery.WithRecoveryHandler(customFunc),
	}
	unaryServerInterceptorBuilder.Use(grpc_recovery.UnaryServerInterceptor(opts...))

}

// OnPreServerStartup ...
func (s *startup) OnPreServerStartup(ctx context.Context) error {
	log := zerolog.Ctx(ctx).With().Str("method", "OnPreServerStartup").Logger()
	ctx, s.cancel = context.WithCancel(ctx)

	s.myEchoServerRuntime = core_echo_runtime.New(myechoserver.NewStartup())
	s.myEchoServerFuture = fluffycore_async.ExecuteWithPromiseAsync(func(promise async.Promise[fluffycore_async.AsyncResponse]) {
		var err error
		defer func() {
			promise.Success(&fluffycore_async.AsyncResponse{
				Message: "End Serve - echoServer",
				Error:   err,
			})
		}()
		log.Info().Msg("echoServer starting up")
		err = s.myEchoServerRuntime.Run()
		if err != nil {
			log.Error().Err(err).Msg("failed to start server")
		}
	})

	s.ddProfiler = di.Get[fluffycore_contracts_ddprofiler.IDataDogProfiler](s.RootContainer)
	s.ddProfiler.Start(ctx)

	s.purger = di.Get[contracts_lockclient.IPurger](s.RootContainer)
	err := s.purger.Start(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to start purger")
		return err
	}
	return nil
}

// OnPreServerShutdown ...
func (s *startup) OnPreServerShutdown(ctx context.Context) {
	log := zerolog.Ctx(ctx).With().Str("method", "OnPreServerShutdown").Logger()
	s.cancel()
	if s.purger != nil {
		log.Info().Msg("purger stopping")
		s.purger.Stop(ctx)
		log.Info().Msg("purger stopped")

	}
	log.Info().Msg("myEchoServerRuntime shutting down")
	s.myEchoServerRuntime.Stop()
	s.myEchoServerFuture.Join()
	log.Info().Msg("myEchoServerRuntime shutdown complete")

	log.Info().Msg("Stopping Datadog Tracer and Profiler")
	s.ddProfiler.Stop(ctx)
	log.Info().Msg("Datadog Tracer and Profiler stopped")
}
