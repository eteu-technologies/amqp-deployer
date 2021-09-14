package main

type DeployMessage struct {
	Tag  string            `json:"tag"`
	Data map[string]string `json:"data"`
}
