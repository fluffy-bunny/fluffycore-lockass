package auth

import (
	proto_lockaas "github.com/fluffy-bunny/fluffycore-lockaas/proto/lockaas"
	contracts_common "github.com/fluffy-bunny/fluffycore/contracts/common"
	services_common_claimsprincipal "github.com/fluffy-bunny/fluffycore/services/common/claimsprincipal"
)

var writeEndpoints = []string{
	//proto_helloworld.Greeter_SayHello_FullMethodName,
}
var noAuthEndpoints = []string{
	"/grpc.health.v1.Health/Check",

	proto_lockaas.Lockaas_ExclusiveLock_FullMethodName,
	proto_lockaas.Lockaas_SharedLock_FullMethodName,
	proto_lockaas.Lockaas_Unlock_FullMethodName,
	proto_lockaas.Lockaas_Renew_FullMethodName,
	proto_lockaas.Lockaas_Status_FullMethodName,
}

func BuildGrpcEntrypointPermissionsClaimsMap() map[string]contracts_common.IEntryPointConfig {
	entryPointClaimsBuilder := services_common_claimsprincipal.NewEntryPointClaimsBuilder()
	for _, endpoint := range noAuthEndpoints {
		entryPointClaimsBuilder.WithGrpcEntrypointPermissionsClaimsMapOpen(endpoint)
	}
	for _, endpoint := range writeEndpoints {
		entrypointConfig := &services_common_claimsprincipal.EntryPointConfig{
			FullMethodName: endpoint,
			ClaimsAST: &services_common_claimsprincipal.ClaimsAST{
				Or: []contracts_common.IClaimsValidator{
					&services_common_claimsprincipal.ClaimsAST{
						ClaimFacts: []contracts_common.IClaimFact{
							services_common_claimsprincipal.NewClaimFact(contracts_common.Claim{
								Type:  "permissions",
								Value: "write",
							}),
						},
					},
				},
			},
		}
		entryPointClaimsBuilder.EntrypointClaimsMap[endpoint] = entrypointConfig
	}
	entryPointClaimsBuilder.DumpExpressions()
	return entryPointClaimsBuilder.GetEntryPointClaimsMap()
}
