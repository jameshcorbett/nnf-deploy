/*
 * Copyright 2021-2023 Hewlett Packard Enterprise Development LP
 * Other additional copyright holders may be indicated within.
 *
 * The entirety of this work is licensed under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 *
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package config

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

var sysCfgPath string

type System struct {
	Name     string                    `yaml:"name"`
	Aliases  []string                  `yaml:"aliases,flow,omitempty"`
	Overlays []string                  `yaml:"overlays,omitempty,flow"`
	Workers  []string                  `yaml:"workers,flow,omitempty"`
	Rabbits  map[string]map[int]string `yaml:"rabbits,flow"`
	Ports    []string                  `yaml:"ports,flow,omitempty"`
	K8sHost  string                    `yaml:"k8sHost,flow,omitempty"`
	K8sPort  string                    `yaml:"k8sPort,flow,omitempty"`
}

type SystemConfigFile struct {
	Systems []System `yaml:"systems"`
}

func FindSystem(name, configPath string) (*System, error) {
	config, err := ReadConfig(configPath)
	if err != nil {
		return nil, err
	}

	for _, system := range config.Systems {
		if system.Name == name {
			return &system, nil
		}
		for _, alias := range system.Aliases {
			if alias == name {
				return &system, nil
			}
		}
	}

	return nil, fmt.Errorf("System '%s' Not Found", name)
}

func ReadConfig(path string) (*SystemConfigFile, error) {
	configFile, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read system config: %v", err)
	}

	config := new(SystemConfigFile)
	if err := yaml.UnmarshalStrict(configFile, config); err != nil {
		return nil, fmt.Errorf("invalid system config yaml: %v", err)
	}

	sysCfgPath = path

	if err := config.Verify(); err != nil {
		return nil, fmt.Errorf("invalid system config: %v", err)
	}

	return config, nil
}

func (config *SystemConfigFile) Verify() error {
	knownNames := make(map[string]bool)
	knownAlias := make(map[string]bool)

	for _, system := range config.Systems {

		// Make sure system names only appear once
		if _, found := knownNames[system.Name]; found {
			return fmt.Errorf("system name '%s' declared more than once in '%s'", system.Name, sysCfgPath)
		}
		knownNames[system.Name] = true

		// Make sure alias only appear once
		for _, alias := range system.Aliases {
			if _, found := knownAlias[alias]; found {
				return fmt.Errorf("alias '%s' declared more than once in '%s'", alias, sysCfgPath)
			}
			knownAlias[alias] = true
		}

		// Verify the individual components in the system (e.g. rabbits, computes)
		if err := system.Verify(); err != nil {
			return err
		}
	}

	return nil
}

func (system *System) Verify() error {
	knownAliases := make(map[string]bool)
	knownOverlays := make(map[string]bool)
	knownComputes := make(map[string]bool)
	knownWorkers := make(map[string]bool)

	if len(system.Rabbits) < 1 {
		return fmt.Errorf("no rabbit nodes declared for system '%s' in '%s'", system.Name, sysCfgPath)
	}

	// Ensure computes are only listed once. `yaml.UnmarshalStrict` will catch duplicate rabbit names since it's a map.
	for _, computes := range system.Rabbits {
		for _, compute := range computes {
			if _, found := knownComputes[compute]; found {
				return fmt.Errorf("compute node '%s' declared more than once for system '%s' in '%s'", compute, system.Name, sysCfgPath)
			}
			knownComputes[compute] = true
		}
	}

	// Aliases
	for _, alias := range system.Aliases {
		if _, found := knownAliases[alias]; found {
			return fmt.Errorf("alias '%s' declared more than once for system '%s' in '%s'", alias, system.Name, sysCfgPath)
		}
		knownAliases[alias] = true
	}

	// Overlays
	if len(system.Overlays) < 1 {
		return fmt.Errorf("no overlays declared for system '%s' in '%s'", system.Name, sysCfgPath)
	}
	for _, overlay := range system.Overlays {
		if _, found := knownOverlays[overlay]; found {
			return fmt.Errorf("overlay'%s' declared more than once for system '%s' in '%s'", overlay, system.Name, sysCfgPath)
		}
		knownOverlays[overlay] = true
	}

	// Workers
	if len(system.Workers) < 1 {
		return fmt.Errorf("no workers declared for system '%s' in '%s'", system.Name, sysCfgPath)
	}
	for _, worker := range system.Workers {
		if _, found := knownWorkers[worker]; found {
			return fmt.Errorf("worker node '%s' declared more than once for system '%s' in '%s'", worker, system.Name, sysCfgPath)
		}
		knownWorkers[worker] = true
	}

	return nil
}

type RepositoryConfigFile struct {
	Repositories       []Repository        `yaml:"repositories"`
	BuildConfig        BuildConfiguration  `yaml:"buildConfiguration"`
	ThirdPartyServices []ThirdPartyService `yaml:"thirdPartyServices"`
}

type Repository struct {
	Name            string   `yaml:"name"`
	Overlays        []string `yaml:"overlays,flow"`
	Development     string   `yaml:"development"`
	Master          string   `yaml:"master"`
	UseRemoteK      bool     `yaml:"useRemoteK,omitempty"`
	RemoteReference struct {
		Build string `yaml:"build"`
		Url   string `yaml:"url"`
	} `yaml:"remoteReference,omitempty"`
}

type BuildConfiguration struct {
	Env []struct {
		Name  string `yaml:"name"`
		Value string `yaml:"value"`
	} `yaml:"env"`
}

type ThirdPartyService struct {
	Name       string `yaml:"name"`
	UseRemoteF bool   `yaml:"useRemoteF,omitempty"`
	Url        string `yaml:"url"`
	WaitCmd    string `yaml:"waitCmd,omitempty"`
}

func readConfigFile(configPath string) (*RepositoryConfigFile, error) {
	configFile, err := os.ReadFile(configPath)
	if err != nil {
		configFile, err = os.ReadFile(filepath.Join("..", configPath))
		if err != nil {
			return nil, err
		}
	}
	config := new(RepositoryConfigFile)
	if err := yaml.UnmarshalStrict(configFile, config); err != nil {
		return nil, err
	}
	return config, nil
}

func FindRepository(configPath string, module string) (*Repository, *BuildConfiguration, error) {

	config, err := readConfigFile(configPath)
	if err != nil {
		return nil, nil, err
	}

	for _, repository := range config.Repositories {
		if module == repository.Name {
			return &repository, &config.BuildConfig, nil
		}
	}

	return nil, nil, fmt.Errorf("Repository '%s' Not Found", module)
}

func GetThirdPartyServices(configPath string) ([]ThirdPartyService, error) {
	config, err := readConfigFile(configPath)
	if err != nil {
		return nil, err
	}
	return config.ThirdPartyServices, nil
}

type Daemon struct {
	Name            string `yaml:"name"`
	Bin             string `yaml:"bin"`
	BuildCmd        string `yaml:"buildCmd"`
	Repository      string `yaml:"repository"`
	Path            string `yaml:"path"`
	SkipNnfNodeName bool   `yaml:"skipNnfNodeName"`
	ServiceAccount  struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	} `yaml:"serviceAccount,omitempty"`
	ExtraArgs string `yaml:"extraArgs,omitempty"`
}

type DaemonConfigFile struct {
	Daemons []Daemon `yaml:"daemons"`
}

func EnumerateDaemons(configPath string, handleFn func(Daemon) error) error {
	configFile, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	config := new(DaemonConfigFile)
	if err := yaml.UnmarshalStrict(configFile, config); err != nil {
		return err
	}

	for _, daemon := range config.Daemons {
		if err := handleFn(daemon); err != nil {
			return err
		}
	}

	return nil
}
