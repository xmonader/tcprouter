package tcprouter

import (
	"context"
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/magiconair/properties/assert"
	"github.com/stretchr/testify/require"
)

func TestEnd2End(t *testing.T) {
	sizes := []int{1, 128}
	for _, size := range sizes {
		name := fmt.Sprintf("%dMiB", size)
		t.Run(name, func(t *testing.T) {
			testEnd2End(t, size)
		})
	}
}
func testEnd2End(t *testing.T, size int) {

	var (
		domain          = "localhost"
		secret          = "foobar"
		httpsPort  uint = 8000
		httpPort   uint = 8001
		clientPort uint = 8002
		body            = make([]byte, size)
	)
	_, err := rand.Read(body)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	wg := sync.WaitGroup{}
	wg.Add(2)

	serverOpts := ServerOptions{
		ListeningAddr:           "0.0.0.0",
		ListeningTLSPort:        httpsPort,
		ListeningHTTPPort:       httpPort,
		ListeningForClientsPort: clientPort,
	}
	s := NewServer(serverOpts, nil, map[string]Service{
		domain: {ClientSecret: secret},
	})

	// start fake local app so client can connect to it
	localApp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))

	// start tcprouter server
	go func() {
		defer wg.Done()
		err = s.Start(ctx)
		require.NoError(t, err)
	}()

	// start tcprouter client
	u, err := url.Parse(localApp.URL)
	require.NoError(t, err)
	go func() {
		defer wg.Done()
		local := u.Host
		remote := fmt.Sprintf("%s:%d", domain, clientPort)
		log.Printf("start client local:%v remote:%v\n", local, remote)
		client := NewClient(secret, local, local, remote)
		client.Start(ctx)
	}()

	// let everything settle
	time.Sleep(time.Second)

	req, err := http.NewRequest("GET", fmt.Sprintf("http://%s:%d", domain, httpPort), nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	if resp.StatusCode != http.StatusOK {
		t.Error("wrong status code received")
	}

	got, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)

	assert.Equal(t, got, body)

	localApp.Close()
	cancel()
	wg.Wait()
}
