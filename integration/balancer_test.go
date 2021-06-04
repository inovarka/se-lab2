package integration

import (
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const baseAddress = "http://balancer:8090"

var client = http.Client{
	Timeout: 3 * time.Second,
}

func TestBalancer(t *testing.T) {
	for i := 0; i < 3; i++ {
		route := fmt.Sprintf("%s/server%s", baseAddress, strconv.Itoa(i))
		resp, err := client.Get(route)
		assert.Nil(t, err)
		compare := resp.Header.Get("lb-from")
		for j := 0; j < 5; j++ {
			resp, err = client.Get(route)
			assert.Equal(t, compare, resp.Header.Get("lb-from"))
			assert.Nil(t, err)
		}
	}
}

func BenchmarkBalancer(b *testing.B) {
	var timeForQueries int64 = 0
	iterations := b.N
	for i := 0; i < 3; i++ {
		route := fmt.Sprintf("%s/server%s", baseAddress, strconv.Itoa(i))
		resp, err := client.Get(route)
		assert.Nil(b, err)
		compare := resp.Header.Get("lb-from")
		for j := 0; j < iterations; j++ {
			start := time.Now()
			resp, err = client.Get(route)
			timeForQueries += time.Since(start).Nanoseconds()
			assert.Equal(b, compare, resp.Header.Get("lb-from"))
			assert.Nil(b, err)
		}
	}
	fmt.Printf("\naverage query time: %s\n", strconv.Itoa(int(timeForQueries)/iterations))
}
