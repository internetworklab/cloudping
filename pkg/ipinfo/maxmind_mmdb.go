package ipinfo

import (
	"context"
	"fmt"
	"net/netip"
	"strings"

	"github.com/oschwald/maxminddb-golang/v2"
)

// GeoNameRecord represents a named geographic entity (e.g. city).
type GeoNameRecord struct {
	GeoNameID uint              `maxminddb:"geoname_id" json:"geoname_id"`
	Names     map[string]string `maxminddb:"names" json:"names"`
}

// ContinentRecord represents a continent entry.
type ContinentRecord struct {
	Code      string            `maxminddb:"code" json:"code"`
	GeoNameID uint              `maxminddb:"geoname_id" json:"geoname_id"`
	Names     map[string]string `maxminddb:"names" json:"names"`
}

// CountryRecord represents a country or subdivision entry with an ISO code.
type CountryRecord struct {
	GeoNameID uint              `maxminddb:"geoname_id" json:"geoname_id"`
	ISOCode   string            `maxminddb:"iso_code" json:"iso_code"`
	Names     map[string]string `maxminddb:"names" json:"names"`
}

// LocationRecord represents geographic coordinates and related metadata.
type LocationRecord struct {
	AccuracyRadius *uint    `maxminddb:"accuracy_radius" json:"accuracy_radius,omitempty"`
	Latitude       *float64 `maxminddb:"latitude" json:"latitude,omitempty"`
	Longitude      *float64 `maxminddb:"longitude" json:"longitude,omitempty"`
	TimeZone       string   `maxminddb:"time_zone" json:"time_zone"`
}

// PostalRecord represents postal information.
type PostalRecord struct {
	Code string `maxminddb:"code" json:"code"`
}

// CityRecord is the top-level record decoded from GeoLite2-City.mmdb.
type CityRecord struct {
	City              GeoNameRecord   `maxminddb:"city" json:"city"`
	Continent         ContinentRecord `maxminddb:"continent" json:"continent"`
	Country           CountryRecord   `maxminddb:"country" json:"country"`
	Location          LocationRecord  `maxminddb:"location" json:"location"`
	Postal            PostalRecord    `maxminddb:"postal" json:"postal"`
	RegisteredCountry CountryRecord   `maxminddb:"registered_country" json:"registered_country"`
	Subdivisions      []CountryRecord `maxminddb:"subdivisions" json:"subdivisions"`
}

// ASNRecord is decoded from GeoLite2-ASN.mmdb.
type ASNRecord struct {
	ASN uint32 `maxminddb:"autonomous_system_number"`
	Org string `maxminddb:"autonomous_system_organization"`
}

// MaxMindMMDBAdapter is a GeneralIPInfoAdapter that resolves IP information
// from local MaxMind GeoLite2 City and ASN MMDB files.
type MaxMindMMDBAdapter struct {
	name   string
	cityDB *maxminddb.Reader
	asnDB  *maxminddb.Reader
}

// GetIPInfo queries both the City and ASN MMDB files for the given IP and
// returns a BasicIPInfo populated with the merged result.
func (ia *MaxMindMMDBAdapter) GetIPInfo(ctx context.Context, ip string) (*BasicIPInfo, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return nil, fmt.Errorf("maxmind: invalid ip address %q: %w", ip, err)
	}

	basicInfo := new(BasicIPInfo)

	// --- ASN lookup ---
	if ia.asnDB != nil {
		var asnRec ASNRecord
		if err := ia.asnDB.Lookup(addr).Decode(&asnRec); err != nil {
			return nil, fmt.Errorf("maxmind: asn lookup failed for %s: %w", ip, err)
		}
		if asnRec.ASN != 0 {
			basicInfo.ASN = fmt.Sprintf("AS%d", asnRec.ASN)
		}
		if asnRec.Org != "" {
			basicInfo.ISP = asnRec.Org
		}
	}

	// --- City lookup ---
	if ia.cityDB != nil {
		var cityRec CityRecord
		if err := ia.cityDB.Lookup(addr).Decode(&cityRec); err != nil {
			return nil, fmt.Errorf("maxmind: city lookup failed for %s: %w", ip, err)
		}

		// Build a human-readable location string from available components.
		parts := make([]string, 0, 3)
		if cityRec.City.Names["en"] != "" {
			parts = append(parts, cityRec.City.Names["en"])
		}
		if len(cityRec.Subdivisions) > 0 && cityRec.Subdivisions[0].Names["en"] != "" {
			parts = append(parts, cityRec.Subdivisions[0].Names["en"])
		}
		if cityRec.Country.Names["en"] != "" {
			parts = append(parts, cityRec.Country.Names["en"])
		}
		if len(parts) > 0 {
			basicInfo.Location = joinLocation(parts)
		}

		// Country name.
		if name := cityRec.Country.Names["en"]; name != "" {
			basicInfo.Country = &name
		}

		// ISO 3166-1 alpha-2 country code.
		if code := cityRec.Country.ISOCode; code != "" {
			basicInfo.ISO3166Alpha2 = &code
		}

		// Region (first subdivision).
		if len(cityRec.Subdivisions) > 0 {
			if sub := cityRec.Subdivisions[0].Names["en"]; sub != "" {
				basicInfo.Region = &sub
			}
		}

		// City.
		if city := cityRec.City.Names["en"]; city != "" {
			basicInfo.City = &city
		}

		// Exact coordinates.
		if cityRec.Location.Latitude != nil && cityRec.Location.Longitude != nil {
			basicInfo.Exact = &ExactLocation{
				Latitude:  *cityRec.Location.Latitude,
				Longitude: *cityRec.Location.Longitude,
			}
		}
	}

	return basicInfo, nil
}

// GetName returns the adapter's display name.
func (ia *MaxMindMMDBAdapter) GetName() string {
	if name := ia.name; name != "" {
		return name
	}
	return "maxmind"
}

// NewMaxMindMMDBAdapter opens the given City and ASN MMDB files and returns a
// ready-to-use MaxMindMMDBAdapter. Either cityMMDBFile or asnMMDBFile may be
// empty if only one database is available, but at least one must be provided.
func NewMaxMindMMDBAdapter(name, cityMMDBFile, asnMMDBFile string) (*MaxMindMMDBAdapter, error) {
	if cityMMDBFile == "" && asnMMDBFile == "" {
		return nil, fmt.Errorf("maxmind: at least one of cityMMDBFile or asnMMDBFile must be specified")
	}

	adapter := &MaxMindMMDBAdapter{name: name}

	if cityMMDBFile != "" {
		db, err := maxminddb.Open(cityMMDBFile)
		if err != nil {
			return nil, fmt.Errorf("maxmind: failed to open city database %q: %w", cityMMDBFile, err)
		}
		adapter.cityDB = db
	}

	if asnMMDBFile != "" {
		db, err := maxminddb.Open(asnMMDBFile)
		if err != nil {
			// Close cityDB if already opened to avoid leaking the file handle.
			if adapter.cityDB != nil {
				adapter.cityDB.Close()
			}
			return nil, fmt.Errorf("maxmind: failed to open asn database %q: %w", asnMMDBFile, err)
		}
		adapter.asnDB = db
	}

	return adapter, nil
}

// joinLocation concatenates location parts with ", ".
func joinLocation(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(parts[0])
	for _, p := range parts[1:] {
		b.WriteString(", ")
		b.WriteString(p)
	}
	return b.String()
}
