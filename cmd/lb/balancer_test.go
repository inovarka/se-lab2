package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBalancer(t *testing.T) {
	var emptyPool []string
	_, emptyErr := NewHealthChecker(&emptyPool)
	assert.EqualError(t, emptyErr, "no addresses given")

	healthPool, initialErr := NewHealthChecker(&serversPool)
	assert.Nil(t, initialErr)
	assert.Equal(t, len(*healthPool), len(serversPool))

	assert.Equal(t, len(healthPool.GetHealthy()), 0)

	// check
	healthTests := map[[3]bool][]string{
		[3]bool{false, false, false}: make([]string, 0),
		[3]bool{true, true, true}:    serversPool,

		[3]bool{true, false, false}: []string{serversPool[0]},
		[3]bool{false, true, false}: []string{serversPool[1]},
		[3]bool{false, false, true}: []string{serversPool[2]},

		[3]bool{true, true, false}: []string{serversPool[0], serversPool[1]},
		[3]bool{true, false, true}: []string{serversPool[0], serversPool[2]},
		[3]bool{false, true, true}: []string{serversPool[1], serversPool[2]},
	}

	for healthStates, expected := range healthTests {
		for i, healthState := range healthStates {
			healthPool.SetHealthState(i, healthState)
		}
		assert.Equal(t, healthPool.GetHealthy(), expected)
	}

	routes := []string{
		"/",
		"/path1",
		"/path1/path2",
		"/path1/path2/path3",
		"/path1/path2/path3/path4",
		"/path1/path2/path3/path4/path5",
	}

	// unhealthy
	healthPool.SetHealthState(0, false)
	healthPool.SetHealthState(1, false)
	healthPool.SetHealthState(2, false)
	_, balancerErr := balance(healthPool, "/some-path")
	assert.EqualError(t, balancerErr, "no servers available")

	checkHashing := func() {
		healthyLen := uint64(len(healthPool.GetHealthy()))
		for _, route := range routes {
			server, _ := balance(healthPool, route)
			expectedIndex := hashPath(route) % healthyLen
			assert.Equal(t, server, serversPool[expectedIndex])
		}
	}

	// healthy
	for i := 0; i < 3; i++ {
		healthPool.SetHealthState(i, true)
		checkHashing()
	}

	n := 10

	resultRoutes := routes[2:5]
	resultRoutesLen := len(resultRoutes)
	resultIndexes := make([]uint, resultRoutesLen)
	for i, route := range resultRoutes {
		index := hashPath(route) % uint64(resultRoutesLen)
		resultIndexes[i] = uint(index)
	}
	// check cycle
	for i := 0; i < n; i++ {
		for u, route := range resultRoutes {
			expectedIndex := resultIndexes[u]
			address, _ := balance(healthPool, route)
			assert.Equal(t, address, serversPool[expectedIndex])
		}
	}
}
