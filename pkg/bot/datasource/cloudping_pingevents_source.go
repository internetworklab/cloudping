package datasource

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	pkgbot "example.com/rbmq-demo/pkg/bot"
	pkgconnreg "example.com/rbmq-demo/pkg/connreg"
	pkgnodereg "example.com/rbmq-demo/pkg/nodereg"
	pkgpinger "example.com/rbmq-demo/pkg/pinger"
	pkgraw "example.com/rbmq-demo/pkg/raw"
)

// CloudPingEventsProvider is an implementation
// of pkgbot.PingEventsProvider interface
type CloudPingEventsProvider struct {
	APIPrefix string
	JWTToken  string
	Resolver  string
}

const defaultPingIntv time.Duration = 1000 * time.Millisecond
const defaultPktTiemoutMs int = 3000
const defaultIPInfoProviderName string = "auto"
const maxPktCountAllowed int = 128

func (provider *CloudPingEventsProvider) GetAuthorizationHeader() string {
	if jwtToken := provider.JWTToken; jwtToken != "" {
		return fmt.Sprintf("bearer %s", jwtToken)
	}
	return ""
}

var ErrReqURLInvalid = errors.New("invalid request url")
var ErrInvalidPingRequest = errors.New("invalid ping request")

func (provider *CloudPingEventsProvider) GetPingURL(pingRequestDesc *pkgbot.PingRequestDescriptor) (*url.URL, error) {
	fullPath := provider.APIPrefix + "/ping"
	urlObj, err := url.Parse(fullPath)
	if err != nil {
		return nil, ErrReqURLInvalid
	}

	pingRequest := pkgpinger.SimplePingRequest{
		From:             pingRequestDesc.Sources,
		Targets:          pingRequestDesc.Destinations,
		IntvMilliseconds: int(defaultPingIntv.Milliseconds()),
	}
	l4Ty := pkgpinger.L4ProtoICMP
	pingRequest.L4PacketType = &l4Ty
	pingRequest.PktTimeoutMilliseconds = defaultPktTiemoutMs
	ipInfoPr := defaultIPInfoProviderName
	pingRequest.IPInfoProviderName = &ipInfoPr

	if pingRequestDesc.Traceroute {
		ttl, _ := pkgpinger.ParseToAutoTTL("auto")
		pingRequest.TTL = ttl
	}

	preferV6 := pingRequestDesc.PreferV6
	preferV4 := pingRequestDesc.PreferV4
	if preferV6 && !preferV4 {
		pingRequest.PreferV6 = &preferV6
	} else if preferV4 && !preferV6 {
		pingRequest.PreferV4 = &preferV4
	}

	totalPkts := pingRequestDesc.Count
	if totalPkts <= 0 || totalPkts > maxPktCountAllowed {
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

func (provider *CloudPingEventsProvider) GetEvents(ctx context.Context, pingRequest *pkgbot.PingRequestDescriptor) <-chan pkgbot.PingEvent {
	dataCh := make(chan pkgbot.PingEvent, 1)
	go func() {
		defer close(dataCh)

		urlObj, err := provider.GetPingURL(pingRequest)
		if err != nil {
			dataCh <- pkgbot.PingEvent{
				Err: fmt.Sprintf("can't get ping event stream URL: %s", err.Error()),
			}
			return
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlObj.String(), nil)
		if err != nil {
			dataCh <- pkgbot.PingEvent{
				Err: fmt.Sprintf("failed to create request: %s", err.Error()),
			}
			return
		}

		if authHeader := provider.GetAuthorizationHeader(); authHeader != "" {
			req.Header.Set("Authorization", authHeader)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			dataCh <- pkgbot.PingEvent{
				Err: fmt.Sprintf("failed to fetch ping events: %s", err.Error()),
			}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			dataCh <- pkgbot.PingEvent{
				Err: fmt.Sprintf("unexpected status code: %d", resp.StatusCode),
			}
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			if err := scanner.Err(); err != nil {
				dataCh <- pkgbot.PingEvent{
					Err: fmt.Sprintf("scanner error: %s", err.Error()),
				}
				return
			}

			line := scanner.Bytes()
			var pingEVObj pkgpinger.PingEvent
			if err := json.Unmarshal(line, &pingEVObj); err != nil {
				dataCh <- pkgbot.PingEvent{
					Err: fmt.Sprintf("failed to parse ping event: %s", err.Error()),
				}
				continue
			}

			if pingEVObj.Err != nil {
				dataCh <- pkgbot.PingEvent{
					Err: *pingEVObj.Err,
				}
				continue
			}

			if pingEVObj.Error != nil {
				dataCh <- pkgbot.PingEvent{
					Err: pingEVObj.Error.Error(),
				}
				continue
			}
			if pingEVObj.Metadata == nil {
				continue
			}
			if tgt := pingEVObj.Metadata["target"]; tgt == "" {
				continue
			}

			botEvent, err := convertPingEventToBotEvent(&pingEVObj)
			if err != nil {
				dataCh <- pkgbot.PingEvent{
					Err: err.Error(),
				}
				continue
			}

			if botEvent != nil {
				dataCh <- *botEvent
			}
		}

		if err := scanner.Err(); err != nil {
			dataCh <- pkgbot.PingEvent{
				Err: fmt.Sprintf("scanner error: %s", err.Error()),
			}
		}
	}()

	return dataCh
}

func convertPingEventToBotEvent(pingEV *pkgpinger.PingEvent) (*pkgbot.PingEvent, error) {
	botEV := pkgbot.PingEvent{}

	if pingEV.Data == nil {
		return &botEV, errors.New("ping event data is nil")
	}

	// Marshal and unmarshal to convert interface{} to ICMPTrackerEntry
	dataBytes, err := json.Marshal(pingEV.Data)
	if err != nil {
		return &botEV, fmt.Errorf("failed to marshal ping event data: %w", err)
	}

	var icmpEntry pkgraw.ICMPTrackerEntry
	if err := json.Unmarshal(dataBytes, &icmpEntry); err != nil {
		return &botEV, fmt.Errorf("failed to unmarshal ping event data: %w", err)
	}

	botEV.Seq = icmpEntry.Seq

	if len(icmpEntry.RTTMilliSecs) > 0 {
		botEV.RTTMs = int(icmpEntry.RTTMilliSecs[0])
	}

	botEV.Timeout = len(icmpEntry.ReceivedAt) == 0

	// OriginTTL is the TTL of the original outbound IP packet
	botEV.OriginTTL = icmpEntry.TTL

	if len(icmpEntry.Raw) > 0 {
		rawEntry := icmpEntry.Raw[0]
		botEV.Peer = rawEntry.Peer
		if len(rawEntry.PeerRDNS) > 0 {
			botEV.PeerRDNS = rawEntry.PeerRDNS[0]
		}
		botEV.IPPacketSize = rawEntry.Size
		botEV.TTL = rawEntry.TTL
		botEV.LastHop = rawEntry.LastHop

		// Extract IP info fields
		if rawEntry.PeerIPInfo != nil {
			ipInfo := rawEntry.PeerIPInfo
			botEV.ASN = ipInfo.ASN
			botEV.ISP = ipInfo.ISP
			if ipInfo.City != nil {
				botEV.City = *ipInfo.City
			}
			if ipInfo.ISO3166Alpha2 != nil {
				botEV.CountryAlpha2 = *ipInfo.ISO3166Alpha2
			}
			if ipInfo.Exact != nil {
				botEV.ExactLocation = ipInfo.Exact
			}
		}

		// Fallback to direct peer fields if PeerIPInfo is not available
		if botEV.ASN == "" && rawEntry.PeerASN != nil {
			botEV.ASN = *rawEntry.PeerASN
		}
		if botEV.ISP == "" && rawEntry.PeerISP != nil {
			botEV.ISP = *rawEntry.PeerISP
		}
		if botEV.ExactLocation == nil && rawEntry.PeerExactLocation != nil {
			botEV.ExactLocation = rawEntry.PeerExactLocation
		}
	}

	return &botEV, nil
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
			Label: strings.ToUpper(*conn.NodeName),
		}

		if conn.Attributes != nil {
			if country, ok := conn.Attributes[pkgnodereg.AttributeKeyCountryCode]; ok {
				loc.Alpha2CountryCode = country
			}
			if city, ok := conn.Attributes[pkgnodereg.AttributeKeyCityName]; ok {
				loc.CityIATACode = city
			}
		}

		locations = append(locations, loc)
	}
	sort.Slice(locations, func(i, j int) bool {
		return strings.Compare(locations[i].Label, locations[j].Label) < 0
	})

	return locations, nil
}
