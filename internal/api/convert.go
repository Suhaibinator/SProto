package api

// convertToModuleVersionInfoType converts a slice of ModuleVersionInfo to ModuleVersionInfoType
func convertToModuleVersionInfoType(infos []ModuleVersionInfo) []ModuleVersionInfoType {
	result := make([]ModuleVersionInfoType, len(infos))
	for i, info := range infos {
		result[i] = ModuleVersionInfoType{
			Namespace:  info.Namespace,
			Name:       info.Name,
			Version:    info.Version,
			ImportPath: info.ImportPath,
		}
	}
	return result
}
