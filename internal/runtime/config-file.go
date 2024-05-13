package runtime

import (
	"context"
	"encoding/json"
	"os"
	"strings"

	contracts_config "github.com/fluffy-bunny/fluffycore-lockaas/internal/contracts/config"
	fluffycore_utils "github.com/fluffy-bunny/fluffycore/utils"
	zerolog "github.com/rs/zerolog"
)

// onLoadCoreConfig will load a file and merge it over the default config
// you can still use ENV variables to replace as well.  i.e. for secrets that only come in that way.
// ---------------------------------------------------------------------
func onLoadCoreConfig(ctx context.Context, coreConfigFilePath string) error {
	log := zerolog.Ctx(ctx).With().Str("method", "onLoadCoreConfig").Logger()
	log.Info().Str("coreConfigFilePath", coreConfigFilePath).Msg("loading core config file")
	fileContent, err := os.ReadFile(coreConfigFilePath)
	if err != nil {
		log.Warn().Err(err).Msg("failed to read coreConfigFilePath - many not be an error")
		return nil
	}
	fixedFileContent := fluffycore_utils.ReplaceEnv(string(fileContent), "${%s}")
	overlay := map[string]interface{}{}

	err = json.NewDecoder(strings.NewReader(fixedFileContent)).Decode(&overlay)
	if err != nil {
		log.Error().Err(err).Msg("failed to unmarshal ragePath")
		return err
	}
	src := map[string]interface{}{}

	err = json.NewDecoder(strings.NewReader(string(contracts_config.ConfigDefaultJSON))).Decode(&src)
	if err != nil {
		log.Error().Err(err).Msg("failed to unmarshal ConfigDefaultJSON")
		return err
	}
	err = fluffycore_utils.ReplaceMergeMap(overlay, src)
	if err != nil {
		log.Error().Err(err).Msg("failed to ReplaceMergeMap")
		return err
	}
	bb, err := json.Marshal(overlay)
	if err != nil {
		log.Error().Err(err).Msg("failed to marshal overlay")
		return err
	}
	contracts_config.ConfigDefaultJSON = bb

	return nil
}
