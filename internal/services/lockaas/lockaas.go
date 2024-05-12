package lockaas

import (
	"context"

	di "github.com/fluffy-bunny/fluffy-dozm-di"
	contracts_lockclient "github.com/fluffy-bunny/fluffycore-lockaas/internal/contracts/lockclient"
	proto_lockaas "github.com/fluffy-bunny/fluffycore-lockaas/proto/lockaas"
	endpoint "github.com/fluffy-bunny/fluffycore/contracts/endpoint"
	fluffycore_utils "github.com/fluffy-bunny/fluffycore/utils"
	status "github.com/gogo/status"
	grpc_gateway_runtime "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	zerolog "github.com/rs/zerolog"
	mongo_lock "github.com/square/mongo-lock"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
)

type (
	service struct {
		proto_lockaas.LockaasFluffyCoreServer

		mongoLockClient contracts_lockclient.IMongoLockClient
	}
)

var (
	stemService = (*service)(nil)
)

func init() {
	var _ proto_lockaas.IFluffyCoreLockaasServer = (*service)(nil)
	var _ endpoint.IEndpointRegistration = (*service)(nil)
}

func (s *service) Ctor(
	mongoLockClient contracts_lockclient.IMongoLockClient,
) proto_lockaas.IFluffyCoreLockaasServer {
	return &service{
		mongoLockClient: mongoLockClient,
	}
}
func (s *service) RegisterFluffyCoreHandler(gwmux *grpc_gateway_runtime.ServeMux, conn *grpc.ClientConn) {
	proto_lockaas.RegisterLockaasHandler(context.Background(), gwmux, conn)
}

func AddLockaasService(builder di.ContainerBuilder) {
	proto_lockaas.AddLockaasServerWithExternalRegistration(builder,
		stemService.Ctor,
		func() endpoint.IEndpointRegistration {
			return &service{}
		})
}
func (s *service) validateExclusiveLockRequest(request *proto_lockaas.ExclusiveLockRequest) error {
	if fluffycore_utils.IsNil(request) {
		return status.Error(codes.InvalidArgument, "request is nil")
	}
	if fluffycore_utils.IsEmptyOrNil(request.LockId) {
		return status.Error(codes.InvalidArgument, "request.LockId is empty")
	}
	if fluffycore_utils.IsEmptyOrNil(request.ResourceName) {
		return status.Error(codes.InvalidArgument, "request.ResourceName is empty")
	}
	if fluffycore_utils.IsEmptyOrNil(request.LockDetails) {
		request.LockDetails = &proto_lockaas.LockDetails{}
	}
	return nil
}
func (s *service) ExclusiveLock(ctx context.Context, request *proto_lockaas.ExclusiveLockRequest) (*proto_lockaas.ExclusiveLockResponse, error) {
	log := zerolog.Ctx(ctx)
	err := s.validateExclusiveLockRequest(request)
	if err != nil {
		return nil, err
	}
	details := mongo_lock.LockDetails{
		Owner:   request.LockDetails.Owner,
		Host:    request.LockDetails.Host,
		Comment: request.LockDetails.Comment,
		TTL:     uint(request.LockDetails.TTLSeconds),
	}
	err = s.mongoLockClient.XLock(ctx,
		request.ResourceName,
		request.LockId,
		details)
	if err != nil {
		log.Error().Err(err).Msg("ExclusiveLock")
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &proto_lockaas.ExclusiveLockResponse{}, nil
}
func (s *service) validateSharedLockRequest(request *proto_lockaas.SharedLockRequest) error {
	if fluffycore_utils.IsNil(request) {
		return status.Error(codes.InvalidArgument, "request is nil")
	}
	if fluffycore_utils.IsEmptyOrNil(request.LockId) {
		return status.Error(codes.InvalidArgument, "request.LockId is empty")
	}
	if fluffycore_utils.IsEmptyOrNil(request.ResourceName) {
		return status.Error(codes.InvalidArgument, "request.ResourceName is empty")
	}
	if fluffycore_utils.IsEmptyOrNil(request.LockDetails) {
		request.LockDetails = &proto_lockaas.LockDetails{}
	}
	if request.MaxConcurrent == 0 {
		return status.Error(codes.InvalidArgument, "request.MaxConcurrent is 0")
	}
	return nil
}
func (s *service) SharedLock(ctx context.Context, request *proto_lockaas.SharedLockRequest) (*proto_lockaas.SharedLockResponse, error) {
	log := zerolog.Ctx(ctx)
	err := s.validateSharedLockRequest(request)
	if err != nil {
		return nil, err
	}
	details := mongo_lock.LockDetails{
		Owner:   request.LockDetails.Owner,
		Host:    request.LockDetails.Host,
		Comment: request.LockDetails.Comment,
		TTL:     uint(request.LockDetails.TTLSeconds),
	}
	err = s.mongoLockClient.SLock(ctx,
		request.ResourceName,
		request.LockId,
		details,
		int(request.MaxConcurrent))
	if err != nil {
		log.Error().Err(err).Msg("SharedLock")
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &proto_lockaas.SharedLockResponse{}, nil
}
func (s *service) validateUnlockRequest(request *proto_lockaas.UnlockRequest) error {
	if fluffycore_utils.IsNil(request) {
		return status.Error(codes.InvalidArgument, "request is nil")
	}
	if fluffycore_utils.IsEmptyOrNil(request.LockId) {
		return status.Error(codes.InvalidArgument, "request.LockId is empty")
	}

	return nil
}
func (s *service) Unlock(ctx context.Context, request *proto_lockaas.UnlockRequest) (*proto_lockaas.UnlockResponse, error) {
	log := zerolog.Ctx(ctx)
	err := s.validateUnlockRequest(request)
	if err != nil {
		return nil, err
	}
	_, err = s.mongoLockClient.Unlock(ctx, request.LockId)
	if err != nil {
		log.Error().Err(err).Msg("Unlock")
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &proto_lockaas.UnlockResponse{}, nil
}
func (s *service) validateLockStatusRequest(request *proto_lockaas.StatusRequest) error {
	if fluffycore_utils.IsNil(request) {
		return status.Error(codes.InvalidArgument, "request is nil")
	}
	if fluffycore_utils.IsNil(request.Filter) {
		return status.Error(codes.InvalidArgument, "request is nil")
	}
	if fluffycore_utils.IsEmptyOrNil(request.Filter.LockId) &&
		fluffycore_utils.IsEmptyOrNil(request.Filter.LockId) &&
		fluffycore_utils.IsEmptyOrNil(request.Filter.Owner) {
		return status.Error(codes.InvalidArgument, "request.Filter.LockId, request.Filter.ResourceName, request.Filter.Owner are all empty")
	}

	return nil
}
func (s *service) Status(ctx context.Context, request *proto_lockaas.StatusRequest) (*proto_lockaas.StatusResponse, error) {
	log := zerolog.Ctx(ctx)
	err := s.validateLockStatusRequest(request)
	if err != nil {
		return nil, err
	}
	filter := mongo_lock.Filter{}
	if fluffycore_utils.IsNotNil(request.Filter.LockId) &&
		fluffycore_utils.IsNotEmptyOrNil(request.Filter.LockId.Value) {
		filter.LockId = request.Filter.LockId.Value
	}
	if fluffycore_utils.IsNotNil(request.Filter.Resource) &&
		fluffycore_utils.IsNotEmptyOrNil(request.Filter.Resource.Value) {
		filter.Resource = request.Filter.Resource.Value
	}
	if fluffycore_utils.IsNotNil(request.Filter.Owner) &&
		fluffycore_utils.IsNotEmptyOrNil(request.Filter.Owner.Value) {
		filter.Owner = request.Filter.Owner.Value
	}
	if fluffycore_utils.IsNotNil(request.Filter.CreatedAfter) {
		filter.CreatedAfter = request.Filter.CreatedAfter.AsTime()
	}
	if fluffycore_utils.IsNotNil(request.Filter.CreatedBefore) {
		filter.CreatedBefore = request.Filter.CreatedBefore.AsTime()
	}
	if fluffycore_utils.IsNotNil(request.Filter.TTLgte) {
		filter.TTLgte = uint(request.Filter.TTLgte.Value)
	}
	if fluffycore_utils.IsNotNil(request.Filter.TTLlt) {
		filter.TTLlt = uint(request.Filter.TTLlt.Value)
	}
	ss, err := s.mongoLockClient.Status(ctx, filter)
	if err != nil {
		log.Error().Err(err).Msg("Status")
		return nil, status.Error(codes.Internal, err.Error())
	}
	response := &proto_lockaas.StatusResponse{}
	for _, s := range ss {
		createdAt := timestamppb.New(s.CreatedAt)
		var renewedAt *timestamppb.Timestamp
		if s.RenewedAt != nil {
			renewedAt = timestamppb.New(*s.RenewedAt)
		}
		response.LockStatus = append(response.LockStatus,
			&proto_lockaas.LockStatus{
				LockId:     s.LockId,
				Resouce:    s.Resource,
				Owner:      s.Owner,
				Host:       s.Host,
				Comment:    s.Comment,
				TTLSeconds: s.TTL,
				CreatedAt:  createdAt,
				RenewedAt:  renewedAt,
			})
	}
	return response, nil
}
func (s *service) validateRenewRequest(request *proto_lockaas.RenewRequest) error {
	if fluffycore_utils.IsNil(request) {
		return status.Error(codes.InvalidArgument, "request is nil")
	}
	if fluffycore_utils.IsNil(request.LockId) {
		return status.Error(codes.InvalidArgument, "request.LockId is nil")
	}

	return nil
}
func (s *service) Renew(ctx context.Context, request *proto_lockaas.RenewRequest) (*proto_lockaas.RenewResponse, error) {
	log := zerolog.Ctx(ctx)
	err := s.validateRenewRequest(request)
	if err != nil {
		return nil, err
	}
	lstatuss, err := s.mongoLockClient.Renew(ctx, request.LockId, uint(request.TTLSeconds))
	if err != nil {
		log.Error().Err(err).Msg("Renew")
		return nil, status.Error(codes.Internal, err.Error())
	}
	response := &proto_lockaas.RenewResponse{}
	for _, lstatus := range lstatuss {
		createdAt := timestamppb.New(lstatus.CreatedAt)
		var renewedAt *timestamppb.Timestamp
		if lstatus.RenewedAt != nil {
			renewedAt = timestamppb.New(*lstatus.RenewedAt)
		}
		response.LockStatus = append(response.LockStatus,
			&proto_lockaas.LockStatus{
				LockId:     lstatus.LockId,
				Resouce:    lstatus.Resource,
				Owner:      lstatus.Owner,
				Host:       lstatus.Host,
				Comment:    lstatus.Comment,
				TTLSeconds: lstatus.TTL,
				CreatedAt:  createdAt,
				RenewedAt:  renewedAt,
			})
	}
	return response, nil
}
