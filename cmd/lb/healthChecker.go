package main

import "errors"

type server struct {
	addr      string
	isHealthy bool
}

type HostsHealth []server

func (h HostsHealth) SetHealthState(index int, state bool) error {
	if index >= len(h) {
		return errors.New("index out of range")
	}
	h[index].isHealthy = state
	return nil
}

func (h HostsHealth) GetHealthy() []string {
	healthy := make([]string, 0)
	for _, host := range h {
		if host.isHealthy {
			healthy = append(healthy, host.addr)
		}
	}
	return healthy
}

func NewHealthChecker(addrs *[]string) (*HostsHealth, error) {
	if len(*addrs) == 0 {
		return nil, errors.New("no addresses given")
	}
	var hosts HostsHealth
	for _, host := range *addrs {
		hosts = append(hosts, server{addr: host})
	}
	return &hosts, nil
}
