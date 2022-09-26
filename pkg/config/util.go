package config

func CheckOwnerInstExist(cfg *Config, ownerName, InstName string) bool {
	for _, owner := range cfg.Owners {
		if owner.Name != ownerName {
			continue
		}

		for _, inst := range owner.Institutions {
			if inst.Name == InstName {
				return true
			}
		}
	}

	return false
}
