package thingscloud

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

const (
	// APIEndpoint is the public culturedcode https endpoint
	APIEndpoint = "https://cloud.culturedcode.com"
)

var (
	// ErrUnauthorized is returned by the API when the credentials are wrong
	ErrUnauthorized = errors.New("unauthorized")
)

// ClientInfo represents the device metadata sent in the things-client-info header.
type ClientInfo struct {
	DeviceModel string `json:"dm"`
	LocalRegion string `json:"lr"`
	NF          bool   `json:"nf"`
	NK          bool   `json:"nk"`
	AppName     string `json:"nn"`
	AppVersion  string `json:"nv"`
	OSName      string `json:"on"`
	OSVersion   string `json:"ov"`
	PrimaryLang string `json:"pl"`
	UserLocale  string `json:"ul"`
}

// DefaultClientInfo returns a ClientInfo with default values matching a typical Mac client.
func DefaultClientInfo() ClientInfo {
	return ClientInfo{
		DeviceModel: "MacBookPro18,3",
		LocalRegion: "US",
		NF:          true,
		NK:          true,
		AppName:     "ThingsMac",
		AppVersion:  "32209501",
		OSName:      "macOS",
		OSVersion:   "15.7.3",
		PrimaryLang: "en-US",
		UserLocale:  "en-Latn-US",
	}
}

// Client is a culturedcode cloud client. It can be used to interact with the
// things cloud to manage your data.
type Client struct {
	Endpoint   string
	EMail      string
	password   string
	ClientInfo ClientInfo
	Debug      bool

	client *http.Client
	common service

	Accounts *AccountService
}

type service struct {
	client *Client
}

// New initializes a things client
func New(endpoint, email, password string) *Client {
	c := &Client{
		Endpoint:   endpoint,
		EMail:      email,
		password:   password,
		ClientInfo: DefaultClientInfo(),

		// A stalled connection must not hang Sync forever; large initial
		// syncs (thousands of items) still finish well within this.
		client: &http.Client{Timeout: 60 * time.Second},
	}
	c.common.client = c
	c.Accounts = (*AccountService)(&c.common)
	return c
}

// ThingsUserAgent is the http user-agent header set by things for mac
const ThingsUserAgent = "ThingsMac/32209501"

// HTTPError is returned when the Things Cloud server responds with an
// unexpected status code. Callers can inspect StatusCode with errors.As
// to decide whether a request is worth retrying.
type HTTPError struct {
	StatusCode int
	Status     string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("http response code: %s", e.Status)
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	if req.Host == "" {
		uri := fmt.Sprintf("%s%s", c.Endpoint, req.URL)
		u, err := url.Parse(uri)
		if err != nil {
			return nil, err
		}
		req.URL = u
	}

	// Common headers matching Things.app
	req.Header.Set("Host", "cloud.culturedcode.com")
	req.Header.Set("User-Agent", ThingsUserAgent)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Charset", "UTF-8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	// Only set Content-Type/Encoding for requests with body (POST, PUT, etc.)
	if req.Method != "GET" && req.Method != "HEAD" && req.Method != "DELETE" {
		req.Header.Set("Content-Type", "application/json; charset=UTF-8")
		req.Header.Set("Content-Encoding", "UTF-8")
	}

	ciJSON, err := json.Marshal(c.ClientInfo)
	if err != nil {
		return nil, fmt.Errorf("marshaling client info: %w", err)
	}
	req.Header.Set("Things-Client-Info", base64.StdEncoding.EncodeToString(ciJSON))

	if c.Debug {
		bs, _ := httputil.DumpRequest(req, true)
		log.Println("REQUEST:", string(bs))
	}

	resp, err := c.client.Do(req)
	if c.Debug {
		if err == nil {
			bs, _ := httputil.DumpResponse(resp, true)
			log.Println("RESPONSE:", string(bs))
		}
		log.Println()
	}
	return resp, err
}
