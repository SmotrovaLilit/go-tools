package main


import (
	"net/http"
	"io"
	"net/url"
	"strings"
	"os/signal"
	"os"
	"syscall"
	"fmt"
)

var proxyHost = "example.com"
var port = 3003

var client =  newHTTPClient();

func handler(w http.ResponseWriter, req *http.Request) {
	var err error
	defer func() {
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(err.Error()))
		}
	}()

	proxyReq, err := http.NewRequest(req.Method, "http://" + proxyHost + req.RequestURI, req.Body)
	if err != nil {
		return
	}

	copyHeaders(proxyReq.Header, req.Header, req.Host)

	resp, err := client.Do(proxyReq)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	w.Header().Add("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
	copyHeaders(w.Header(), resp.Header, req.Host)
	w.WriteHeader(resp.StatusCode)

	_, err = io.Copy(w, resp.Body)
	if err != nil {
		if err == io.EOF {
			err = nil
		}

		return
	}
}

func copyHeaders(out http.Header, in http.Header, host string) {
	var hopHeaders = []string{
		"Connection",
		"Proxy-Connection", // non-standard but still sent by libcurl and rejected by e.g. google
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"Te",      // canonicalized version of "TE"
		"Trailer", // not Trailers per URL above; http://www.rfc-editor.org/errata_search.php?eid=4522
		"Transfer-Encoding",
		"Upgrade",
	}

	for _, header := range hopHeaders {
		in.Del(header)
	}

	for key, values := range in {
		for _, value := range values {
			if key == "Set-Cookie" {

			}
			out.Set(key, strings.Replace(value, proxyHost, host, -1))
		}
	}
}

func newHTTPClient() *http.Client {
	proxyUrl, err := url.Parse("http://" + proxyHost)
	if err != nil {
		panic(err)
	}

	client := &http.Client{}
	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyUrl),
		MaxIdleConnsPerHost: 100,
		MaxIdleConns:        100,
	}

	client.Transport = transport

	return client
}

func main()  {
	go func() {
		err := http.ListenAndServe(fmt.Sprintf(":%d", port), http.HandlerFunc(handler))
		if err != nil {
			panic(err)
		}
	}()

	sigs := make(chan os.Signal)
	signal.Notify(sigs, syscall.SIGHUP)

	for range sigs {
		continue
	}
}
