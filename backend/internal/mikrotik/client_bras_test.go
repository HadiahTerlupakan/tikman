package mikrotik

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// restClient serves handler as a pinned REST router.
func restClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server, host, port := tlsRouter(t, handler)
	return openREST(t, host, port, "tikman", "secret", Fingerprint(server.Certificate().Raw))
}

func apiClient(t *testing.T, replies map[string][][]string) (*Client, *fakeAPIRouter) {
	t.Helper()
	router := &fakeAPIRouter{username: "tikman", password: "secret", replies: replies}
	client, err := Open(apiConfig(router.start(t, nil)))
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	<-router.received // the login
	return client, router
}

func TestCountParsesEveryShape(t *testing.T) {
	for name, body := range map[string]string{"array": `[{"ret":"2041"}]`, "object": `{"ret":"2041"}`} {
		t.Run("REST "+name, func(t *testing.T) {
			client := restClient(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) })
			n, err := client.PPPActiveCount()
			require.NoError(t, err)
			assert.Equal(t, 2041, n)
		})
	}
	t.Run("API", func(t *testing.T) {
		client, _ := apiClient(t, map[string][][]string{"/ppp/active/print": {{"!done", "=ret=2041"}}})
		n, err := client.PPPActiveCount()
		require.NoError(t, err)
		assert.Equal(t, 2041, n)
	})
	t.Run("REST empty reply is an error, not zero", func(t *testing.T) {
		client := restClient(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`[]`)) })
		_, err := client.PPPActiveCount()
		assert.Error(t, err)
	})
	t.Run("API empty reply is an error, not zero", func(t *testing.T) {
		client, _ := apiClient(t, map[string][][]string{"/ppp/active/print": {{"!done"}}})
		_, err := client.PPPActiveCount()
		assert.Error(t, err)
	})
}

func TestCountSendsQueryAndCountOnly(t *testing.T) {
	var method, path string
	var body map[string]interface{}
	client := restClient(t, func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(raw, &body))
		_, _ = w.Write([]byte(`[{"ret":"7"}]`))
	})
	n, err := client.PoolUsedCount("pool-a")
	require.NoError(t, err)
	assert.Equal(t, 7, n)
	assert.Equal(t, http.MethodPost, method)
	assert.Equal(t, "/rest/ip/pool/used/print", path)
	assert.Equal(t, map[string]interface{}{"count-only": "", ".query": []interface{}{"pool=pool-a"}}, body)

	apiC, router := apiClient(t, map[string][][]string{"/ip/pool/used/print": {{"!done", "=ret=7"}}})
	_, err = apiC.PoolUsedCount("pool-a")
	require.NoError(t, err)
	assert.Equal(t, []string{"/ip/pool/used/print", "=count-only=", "?pool=pool-a"}, <-router.received)
}

func TestRadiusMonitorSendsOnce(t *testing.T) {
	const reply = `[{"pending":"0","requests":"90","accepts":"80","rejects":"5","resends":"3",
		"timeouts":"2","bad-replies":"1","last-request-rtt":"4ms"}]`
	want := RadiusStats{Pending: "0", Requests: "90", Accepts: "80", Rejects: "5", Resends: "3",
		Timeouts: "2", BadReplies: "1", LastRequestRTT: "4ms"}

	var path, body string
	client := restClient(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		body = string(raw)
		_, _ = w.Write([]byte(reply))
	})
	got, err := client.RadiusMonitor("*1")
	require.NoError(t, err)
	assert.Equal(t, want, got)
	assert.Equal(t, "/rest/radius/monitor", path)
	assert.JSONEq(t, `{"numbers":"*1","once":""}`, body)

	apiC, router := apiClient(t, map[string][][]string{"/radius/monitor": {{"!re", "=pending=0", "=requests=90",
		"=accepts=80", "=rejects=5", "=resends=3", "=timeouts=2", "=bad-replies=1", "=last-request-rtt=4ms"}, {"!done"}}})
	got, err = apiC.RadiusMonitor("*1")
	require.NoError(t, err)
	assert.Equal(t, want, got)
	assert.Equal(t, []string{"/radius/monitor", "=numbers=*1", "=once="}, <-router.received)
}

func TestDynamicQueuesAsksForProplist(t *testing.T) {
	client := restClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/rest/queue/simple", r.URL.Path)
		assert.Equal(t, "name,target,rate", r.URL.Query().Get(".proplist"))
		assert.Equal(t, "true", r.URL.Query().Get("dynamic"))
		_, _ = w.Write([]byte(`[{"name":"<pppoe-pel-0001>","target":"192.0.2.10/32","rate":"1000/2000"}]`))
	})
	got, err := client.DynamicQueues()
	require.NoError(t, err)
	assert.Equal(t, []SimpleQueue{{Name: "<pppoe-pel-0001>", Target: "192.0.2.10/32", Rate: "1000/2000"}}, got)

	apiC, router := apiClient(t, map[string][][]string{"/queue/simple/print": {{"!done"}}})
	_, err = apiC.DynamicQueues()
	require.NoError(t, err)
	assert.Equal(t, []string{"/queue/simple/print", "=.proplist=name,target,rate", "?dynamic=true"}, <-router.received)
}

func TestInterfacesAsksForEveryInterface(t *testing.T) {
	client := restClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.False(t, r.URL.Query().Has("dynamic"), "a dynamic=false filter dropped real ports on RouterOS")
		_, _ = w.Write([]byte(`[]`))
	})
	_, err := client.Interfaces()
	require.NoError(t, err)

	apiC, router := apiClient(t, map[string][][]string{"/interface/print": {{"!done"}}})
	_, err = apiC.Interfaces()
	require.NoError(t, err)
	assert.Equal(t, []string{"/interface/print"}, <-router.received)
}

func TestDefaultRouteFilter(t *testing.T) {
	client := restClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		assert.Equal(t, "0.0.0.0/0", q.Get("dst-address"))
		assert.Equal(t, "true", q.Get("active"))
		assert.False(t, q.Has("static"), "a default route learned over BGP counts too")
		_, _ = w.Write([]byte(`[{"dst-address":"0.0.0.0/0","gateway":"192.0.2.1","active":"true"}]`))
	})
	got, err := client.DefaultRoute()
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "192.0.2.1", got[0].Gateway)

	apiC, router := apiClient(t, map[string][][]string{"/ip/route/print": {{"!done"}}})
	_, err = apiC.DefaultRoute()
	require.NoError(t, err)
	assert.Equal(t, []string{"/ip/route/print", "?dst-address=0.0.0.0/0", "?active=true"}, <-router.received)
}
