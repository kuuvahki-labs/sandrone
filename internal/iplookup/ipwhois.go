// Package iplookup provides the external public-IP attribution boundary used
// by node inspection.
package iplookup

import (
	"context"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

const (
	ipwhoisBaseURL   = "https://ipwho.is/" // #nosec G101 -- public provider endpoint, not a credential
	ipwhoisFields    = "ip,success,message,continent,continent_code,country,country_code,connection.asn,connection.org,connection.domain"
	maxResponseBytes = 64 << 10
)

type Attribution struct {
	CountryCode   string `json:"country_code"`
	Country       string `json:"country"`
	ContinentCode string `json:"continent_code"`
	Continent     string `json:"continent"`
	ASN           string `json:"asn"`
	ASName        string `json:"as_name"`
	ASDomain      string `json:"as_domain"`
}

type Provider interface {
	Lookup(context.Context, netip.Addr) (Attribution, error)
}

type Client struct {
	endpoint *url.URL
	http     *http.Client
}

func NewIPWhois() *Client {
	endpoint, err := url.Parse(ipwhoisBaseURL)
	if err != nil {
		panic(err)
	}
	return &Client{
		endpoint: endpoint,
		http:     &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *Client) Lookup(ctx context.Context, ip netip.Addr) (Attribution, error) {
	if c == nil {
		return Attribution{}, errors.New("ipwho.is client is nil")
	}
	if !ip.IsValid() {
		return Attribution{}, errors.New("IP address is invalid")
	}
	requestURL := c.endpoint.JoinPath(ip.Unmap().String())
	query := requestURL.Query()
	query.Set("fields", ipwhoisFields)
	requestURL.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return Attribution{}, errors.New("build ipwho.is request failed")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Attribution{}, ctxErr
		}
		return Attribution{}, errors.New("ipwho.is request failed")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return Attribution{}, fmt.Errorf("ipwho.is returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return Attribution{}, fmt.Errorf("read ipwho.is response: %w", err)
	}
	if len(body) > maxResponseBytes {
		return Attribution{}, errors.New("ipwho.is response is too large")
	}
	var payload struct {
		IP            string `json:"ip"`
		Success       bool   `json:"success"`
		CountryCode   string `json:"country_code"`
		Country       string `json:"country"`
		ContinentCode string `json:"continent_code"`
		Continent     string `json:"continent"`
		Connection    struct {
			ASN    int    `json:"asn"`
			Org    string `json:"org"`
			Domain string `json:"domain"`
		} `json:"connection"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return Attribution{}, fmt.Errorf("decode ipwho.is response: %w", err)
	}
	returnedIP, err := netip.ParseAddr(payload.IP)
	if err != nil || returnedIP.Unmap() != ip.Unmap() || !payload.Success || payload.CountryCode == "" || payload.Country == "" ||
		payload.ContinentCode == "" || payload.Continent == "" || payload.Connection.ASN <= 0 {
		return Attribution{}, errors.New("ipwho.is response is incomplete")
	}
	return Attribution{
		CountryCode:   payload.CountryCode,
		Country:       payload.Country,
		ContinentCode: payload.ContinentCode,
		Continent:     payload.Continent,
		ASN:           fmt.Sprintf("AS%d", payload.Connection.ASN),
		ASName:        strings.TrimSpace(payload.Connection.Org),
		ASDomain:      strings.TrimSpace(payload.Connection.Domain),
	}, nil
}
