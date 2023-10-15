package asset

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/xiaomi388/beancount-automation/pkg/config"
)

// TODO: implement this
func LoadOrInit(dbPath string, cfgOwners []config.Owner) (Asset, error) {
	initedAsset := initAsset()
	loadedAsset, err := load(dbPath)
	if err != nil {
		return fmt.Errorf()
	}

}

func merge(initedAsset, loadedAsset Asset) Asset {

}

func initAsset(cfgOwners []config.Owner) Asset {
	asset := Asset{}

	for _, cfgOwner := range cfgOwners {
		owner := Owner{
			Name: cfgOwner.Name,
		}

		for _, cfgInst := range cfgOwner.Institutions {
			owner.Institution = append(owner.Institution, Institution{
				Name: cfgInst.Name,
			})
		}

		asset.Owners = append(asset.Owners, Owner{
			Name: owner.Name,
		})
	}

	return asset
}

func load(dbPath string) (Asset, error) {
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return Asset{}, nil
	}

	data, err := os.ReadFile(dbPath)
	if err != nil {
		return Asset{}, fmt.Errorf("failed to read db file: %w", err)
	}

	var asset Asset
	if err := json.Unmarshal(data, &asset); err != nil {
		return Asset{}, fmt.Errorf("failed to unmarshal data: %w", err)
	}

	return asset, nil
}
