package datasource

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	pkgbot "example.com/rbmq-demo/pkg/bot"
	pkgconnreg "example.com/rbmq-demo/pkg/connreg"
	pkgpinger "example.com/rbmq-demo/pkg/pinger"
)

// CloudPingEventsProvider is an implementation
// of pkgbot.PingEventsProvider interface
type CloudPingEventsProvider struct {
	APIPrefix   string
	JWTToken    string
	PacketCount int
	Resolver    string
}

const defaultPktsCount int = 10

func (provider *CloudPingEventsProvider) GetPktCount() int {
	if provider.PacketCount != 0 {
		return provider.PacketCount
	}
	return defaultPktsCount
}

func (provider *CloudPingEventsProvider) GetAuthorizationHeader() string {
	if jwtToken := provider.JWTToken; jwtToken != "" {
		return fmt.Sprintf("bearer %s", jwtToken)
	}
	return ""
}

var ErrReqURLInvalid = errors.New("invalid request url")
var ErrInvalidPingRequest = errors.New("invalid ping request")

type PingRequestDescriptor struct {
	Sources      []string
	Destinations []string
}

func (provider *CloudPingEventsProvider) GetPingURL(pingRequestDesc PingRequestDescriptor) (*url.URL, error) {
	fullPath := provider.APIPrefix + "/ping"
	urlObj, err := url.Parse(fullPath)
	if err != nil {
		return nil, ErrReqURLInvalid
	}

	pingRequest := pkgpinger.SimplePingRequest{
		From:    pingRequestDesc.Sources,
		Targets: pingRequestDesc.Destinations,
	}
	totalPkts := provider.GetPktCount()
	if totalPkts == 0 || totalPkts > 10 {
		return nil, ErrInvalidPingRequest
	}
	pingRequest.TotalPkts = &totalPkts

	resolver := provider.Resolver
	if resolver == "" {
		return nil, ErrInvalidPingRequest
	}
	pingRequest.Resolver = &resolver

	urlObj.RawQuery = pingRequest.ToURLValues().Encode()

	return urlObj, nil
}

func (provider *CloudPingEventsProvider) GetLocationsURL() (*url.URL, error) {
	fullPath := provider.APIPrefix + "/conns"
	urlObj, err := url.Parse(fullPath)
	if err != nil {
		return nil, ErrReqURLInvalid
	}
	return urlObj, nil
}

func (provider *CloudPingEventsProvider) GetEventsByLocationCodeAndDestination(ctx context.Context, code string, destination string) <-chan pkgbot.PingEvent {
	dataCh := make(chan pkgbot.PingEvent, 1)
	go func() {
		defer close(dataCh)
		pingRequest := PingRequestDescriptor{
			Sources:      []string{code},
			Destinations: []string{destination},
		}
		urlObj, err := provider.GetPingURL(pingRequest)
		if err != nil {
			dataCh <- pkgbot.PingEvent{
				Err: fmt.Sprintf("can't get ping event stream URL: %s", err.Error()),
			}
			return
		}
	}()

	return dataCh
}

func (provider *CloudPingEventsProvider) GetAllLocations(ctx context.Context) ([]pkgbot.LocationDescriptor, error) {
	urlObj, err := provider.GetLocationsURL()
	if err != nil {
		return nil, errors.New("can't get locations from upstream data provider")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlObj.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if authHeader := provider.GetAuthorizationHeader(); authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch locations: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var conns map[string]*pkgconnreg.ConnRegistryData
	if err := json.Unmarshal(body, &conns); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	locations := make([]pkgbot.LocationDescriptor, 0, len(conns))
	for _, conn := range conns {
		if conn.NodeName == nil {
			continue
		}

		loc := pkgbot.LocationDescriptor{
			Id:    *conn.NodeName,
			Label: *conn.NodeName,
		}

		if conn.Attributes != nil {
			if country, ok := conn.Attributes["country_code"]; ok {
				loc.Alpha2CountryCode = country
			}
			if city, ok := conn.Attributes["city_iata"]; ok {
				loc.CityIATACode = city
			}
		}

		locations = append(locations, loc)
	}

	return locations, nil
}
