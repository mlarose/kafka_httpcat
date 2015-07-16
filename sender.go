package kafka_httpcat

import (
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"time"
)

type HTTPSender struct {
	hosts             []string
	contextPath       string
	method            string
	parsedContextPath *url.URL
	headers           map[string][]string
	currentHost       int
	expectedRespCodes map[int]bool

	client *http.Client
}

func NewHTTPSender(hosts []string, contextPath string, method string, headers map[string][]string, expectedRespCodes []int) *HTTPSender {
	//Random host
	if len(hosts) == 0 {
		log.Fatal("Need at least one host defined.")
	}
	cur := rand.Int() % len(hosts)
	if u, err := url.Parse(contextPath); err != nil {
		log.Fatalf("Unable to parse context path: %s", err)
		return nil
	} else {
		u.Scheme = "http"
		respMap := make(map[int]bool)
		for _, expectedRespCode := range expectedRespCodes {
			respMap[expectedRespCode] = true
		}
		return &HTTPSender{hosts: hosts, contextPath: contextPath, method: method, headers: headers,
			parsedContextPath: u, expectedRespCodes: respMap, currentHost: cur, client: &http.Client{}}
	}
}

func (h *HTTPSender) buildBaseRequest(contextPath string, method string, headers map[string][]string, bodyReader io.ReadCloser) *http.Request {
	var req http.Request
	req.Method = h.method
	req.ProtoMajor = 1
	req.ProtoMinor = 1
	req.Close = false
	req.Header = h.headers
	req.URL = h.parsedContextPath
	req.URL.Host = h.hosts[h.currentHost]
	req.Body = bodyReader
	return &req
}

func (h *HTTPSender) send(bodyReader io.ReadCloser) error {
	req := h.buildBaseRequest(h.contextPath, h.method, h.headers, bodyReader)
	if resp, err := h.client.Do(req); err != nil {
		log.Printf("inot sent!: %s", err)
		return err
	} else {
		if _, ok := h.expectedRespCodes[resp.StatusCode]; !ok {
			return fmt.Errorf("Unexpected http code: %d", resp.StatusCode)
		}
	}

	return nil
}

func (h *HTTPSender) RRSend(bodyReader io.ReadCloser) error {
	retries := 0
	gzreader, err := gzip.NewReader(bodyReader)
	if err != nil {
		log.Fatal("Unable to uncompress payload")
	}

	for {
		if err := h.send(gzreader); err != nil {
			//Round robin
			h.currentHost = (h.currentHost + 1) % len(h.hosts)

			retries++
			if retries > 10 {
				time.Sleep(time.Second)
			}
		} else {
			gzreader.Reset(bodyReader)
			return nil
		}
	}
}
