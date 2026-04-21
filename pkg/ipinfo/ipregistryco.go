package ipinfo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type IPRegistryCOResponse struct {
	Carrier    *IPRegistryCarrier    `json:"carrier"`
	Company    *IPRegistryCompany    `json:"company"`
	Connection *IPRegistryConnection `json:"connection"`
	Currency   *IPRegistryCurrency   `json:"currency"`
	Hostname   *string               `json:"hostname"`
	IP         string                `json:"ip"`
	Location   *IPRegistryLocation   `json:"location"`
	Security   *IPRegistrySecurity   `json:"security"`
	TimeZone   *IPRegistryTimeZone   `json:"time_zone"`
	Type       string                `json:"type"`
	UserAgent  *IPRegistryUserAgent  `json:"user_agent,omitempty"`
}

type IPRegistryCarrier struct {
	Name *string `json:"name"`
	MCC  *string `json:"mcc"`
	MNC  *string `json:"mnc"`
}

type IPRegistryCompany struct {
	Domain *string `json:"domain"`
	Name   string  `json:"name"`
	Type   string  `json:"type"`
}

type IPRegistryConnection struct {
	ASN          int    `json:"asn"`
	Domain       string `json:"domain"`
	Organization string `json:"organization"`
	Route        string `json:"route"`
	Type         string `json:"type"`
}

type IPRegistryCurrency struct {
	Code         string                    `json:"code"`
	Name         string                    `json:"name"`
	NameNative   string                    `json:"name_native"`
	Plural       string                    `json:"plural"`
	PluralNative string                    `json:"plural_native"`
	Symbol       string                    `json:"symbol"`
	SymbolNative string                    `json:"symbol_native"`
	Format       *IPRegistryCurrencyFormat `json:"format"`
}

type IPRegistryCurrencyFormat struct {
	DecimalSeparator string                        `json:"decimal_separator"`
	GroupSeparator   string                        `json:"group_separator"`
	Negative         *IPRegistryCurrencySignFormat `json:"negative"`
	Positive         *IPRegistryCurrencySignFormat `json:"positive"`
}

type IPRegistryCurrencySignFormat struct {
	Prefix string `json:"prefix"`
	Suffix string `json:"suffix"`
}

type IPRegistryLocation struct {
	Continent *IPRegistryContinent `json:"continent"`
	Country   *IPRegistryCountry   `json:"country"`
	Region    *IPRegistryRegion    `json:"region"`
	City      string               `json:"city"`
	Postal    string               `json:"postal"`
	Latitude  float64              `json:"latitude"`
	Longitude float64              `json:"longitude"`
	Language  *IPRegistryLanguage  `json:"language"`
	InEU      bool                 `json:"in_eu"`
}

type IPRegistryContinent struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type IPRegistryCountry struct {
	Area              int                  `json:"area"`
	Borders           []string             `json:"borders"`
	CallingCode       string               `json:"calling_code"`
	Capital           string               `json:"capital"`
	Code              string               `json:"code"`
	Name              string               `json:"name"`
	Population        int                  `json:"population"`
	PopulationDensity float64              `json:"population_density"`
	Flag              *IPRegistryFlag      `json:"flag"`
	Languages         []IPRegistryLanguage `json:"languages"`
	TLD               string               `json:"tld"`
}

type IPRegistryFlag struct {
	Emoji        string `json:"emoji"`
	EmojiUnicode string `json:"emoji_unicode"`
	Emojitwo     string `json:"emojitwo"`
	Noto         string `json:"noto"`
	Twemoji      string `json:"twemoji"`
	Wikimedia    string `json:"wikimedia"`
}

type IPRegistryLanguage struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Native string `json:"native"`
}

type IPRegistryRegion struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type IPRegistrySecurity struct {
	IsAbuser        bool `json:"is_abuser"`
	IsAttacker      bool `json:"is_attacker"`
	IsBogon         bool `json:"is_bogon"`
	IsCloudProvider bool `json:"is_cloud_provider"`
	IsProxy         bool `json:"is_proxy"`
	IsRelay         bool `json:"is_relay"`
	IsTor           bool `json:"is_tor"`
	IsTorExit       bool `json:"is_tor_exit"`
	IsVPN           bool `json:"is_vpn"`
	IsAnonymous     bool `json:"is_anonymous"`
	IsThreat        bool `json:"is_threat"`
}

type IPRegistryTimeZone struct {
	ID               string `json:"id"`
	Abbreviation     string `json:"abbreviation"`
	CurrentTime      string `json:"current_time"`
	Name             string `json:"name"`
	Offset           int    `json:"offset"`
	InDaylightSaving bool   `json:"in_daylight_saving"`
}

type IPRegistryUserAgent struct {
	Header       string            `json:"header"`
	Name         string            `json:"name"`
	Type         string            `json:"type"`
	Version      string            `json:"version"`
	VersionMajor string            `json:"version_major"`
	Device       *IPRegistryDevice `json:"device"`
	Engine       *IPRegistryEngine `json:"engine"`
	OS           *IPRegistryOS     `json:"os"`
}

type IPRegistryDevice struct {
	Brand string `json:"brand"`
	Name  string `json:"name"`
	Type  string `json:"type"`
}

type IPRegistryEngine struct {
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	Version      *string `json:"version"`
	VersionMajor string  `json:"version_major"`
}

type IPRegistryOS struct {
	Name    string  `json:"name"`
	Type    string  `json:"type"`
	Version *string `json:"version"`
}

// `IPRegistryAdapter` is an implementation of `GeneralIPInfoAdapter` interface
type IPRegistryAdapter struct {
	Name              string
	APIEndpoint       string
	APIKey            string
	AdditionalHeaders http.Header
}

// this function takes an non-nil `IPRegistryCOResponse`, returns a `*BasicIPInfo`.
func (adapter *IPRegistryAdapter) getIPInfoFromIPRegistryCOResponse(resp IPRegistryCOResponse) (*BasicIPInfo, error) {
	basicInfo := new(BasicIPInfo)

	if resp.Connection != nil {
		basicInfo.ASN = "AS" + strconv.Itoa(resp.Connection.ASN)
		basicInfo.ISP = resp.Connection.Organization
	}

	locations := make([]string, 0)
	if resp.Location != nil {
		if resp.Location.City != "" {
			locations = append(locations, resp.Location.City)
			basicInfo.City = new(string)
			*basicInfo.City = resp.Location.City
		}
		if resp.Location.Region != nil && resp.Location.Region.Name != "" {
			locations = append(locations, resp.Location.Region.Name)
			basicInfo.Region = new(string)
			*basicInfo.Region = resp.Location.Region.Name
		}
		if resp.Location.Country != nil {
			if resp.Location.Country.Name != "" {
				locations = append(locations, resp.Location.Country.Name)
				basicInfo.Country = new(string)
				*basicInfo.Country = resp.Location.Country.Name
			}
			if resp.Location.Country.Code != "" {
				basicInfo.ISO3166Alpha2 = new(string)
				*basicInfo.ISO3166Alpha2 = resp.Location.Country.Code
			}
		}
		basicInfo.Exact = &ExactLocation{
			Latitude:  resp.Location.Latitude,
			Longitude: resp.Location.Longitude,
		}
	}

	if len(locations) > 0 {
		basicInfo.Location = strings.Join(locations, ", ")
	}

	return basicInfo, nil
}

// return ipinfo for the querying ip address
func (adapter *IPRegistryAdapter) GetIPInfo(ctx context.Context, ip string) (*BasicIPInfo, error) {
	urlObj, err := adapter.GetFullURL(ip, adapter.APIKey)
	if err != nil {
		return nil, fmt.Errorf("Failed to get full URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlObj.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to create request: %w", err)
	}
	if addHeader := adapter.AdditionalHeaders; addHeader != nil {
		if req.Header == nil {
			req.Header = make(http.Header)
		}
		for k, vals := range addHeader {
			for _, v := range vals {
				req.Header.Add(k, v)
			}
		}
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Failed to get IP info: %w", err)
	}

	if resp.StatusCode >= 400 || resp.StatusCode < 200 {
		return nil, fmt.Errorf("IPRegistry returned status code %d", resp.StatusCode)
	}

	defer resp.Body.Close()
	ipregistryResp := &IPRegistryCOResponse{}
	if err := json.NewDecoder(resp.Body).Decode(ipregistryResp); err != nil {
		return nil, fmt.Errorf("Failed to decode IPRegistry response: %w", err)
	}

	return adapter.getIPInfoFromIPRegistryCOResponse(*ipregistryResp)
}

func (adapter *IPRegistryAdapter) getAPIEndpoint() string {
	if endpoint := adapter.APIEndpoint; endpoint != "" {
		return endpoint
	}
	return "https://api.ipregistry.co"
}

func (adapter *IPRegistryAdapter) GetFullURL(ip string, apiKey string) (*url.URL, error) {
	urlStr := adapter.getAPIEndpoint() + "/" + ip
	urlObj, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse raw URL: %w", err)
	}

	if apiKey != "" {
		urlParam := url.Values{}
		urlParam.Set("key", apiKey)
		urlObj.RawQuery = urlParam.Encode()
	}

	return urlObj, nil
}

// return the name of the ipinfo adapter, although mostly there's only one adapter for each ipinfo provider
func (adapter *IPRegistryAdapter) GetName() string {
	return adapter.Name
}
