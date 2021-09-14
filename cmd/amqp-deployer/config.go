package main

type DeployerConfig struct {
	Deployables []Deployable `yaml:"deployables"`

	DeployablesByTag map[string]Deployable `yaml:"-"`
}

type Deployable struct {
	Tag          string               `yaml:"tag"`
	RequiredData []string             `yaml:"required-data"`
	Actions      []DeployableCommands `yaml:"actions"`
}

type DeployableCommands struct {
	WorkDir string   `yaml:"work-dir"`
	Command []string `yaml:"command"`
}
