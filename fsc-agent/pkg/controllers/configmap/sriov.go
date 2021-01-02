package configmap

import (
	"encoding/json"
)

// Config struct
type Config struct {
	ResourceList []struct {
		ResourceName   string `json:"resourceName"`
		ResourcePrefix string `json:"resourcePrefix"`
		Selectors      struct {
			PfNames []string `json:"pfNames"`
		} `json:"selectors"`
	} `json:"resourceList"`
}

// sriovParseConfig function
func sriovParseConfig(data string, sriovConfig *map[string]*string) (error) {
	var c Config

	err := json.Unmarshal([]byte(data), &c)
	if err != nil {
		return err
	}
	var resourceName string
	for _, v := range c.ResourceList {
		resourceName = v.ResourcePrefix + "/" + v.ResourceName
		for _, n := range v.Selectors.PfNames {
			// n is interfaceName
			(*sriovConfig)[resourceName] = &n
		}
	}

	return nil
}
