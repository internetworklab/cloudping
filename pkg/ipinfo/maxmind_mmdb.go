package ipinfo

import (
	"context"
	"fmt"
	"log"
	"net/netip"
	"strings"

	"github.com/fsnotify/fsnotify"
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
	name       string
	cityDB     *maxminddb.Reader
	asnDB      *maxminddb.Reader
	cityDBPath string
	asnDBPath  string
	asnDBChan  chan *maxminddb.Reader
	cityDBChan chan *maxminddb.Reader
	errChan    chan error
}

func (ia *MaxMindMMDBAdapter) Run(ctx context.Context) {
	go ia.doRun(ctx)
}

func (ia *MaxMindMMDBAdapter) loadDBs() error {

	var err error
	ia.cityDB, err = maxminddb.Open(ia.cityDBPath)
	if err != nil {
		return fmt.Errorf("maxmind: failed to open city database %q: %w", ia.cityDBPath, err)
	}

	ia.asnDB, err = maxminddb.Open(ia.asnDBPath)
	if err != nil {
		return fmt.Errorf("maxmind: failed to open asn database %q: %w", ia.asnDBPath, err)
	}

	return nil
}

func (ia *MaxMindMMDBAdapter) doRun(ctx context.Context) {

	defer func() {
		if ia.cityDB != nil {
			ia.cityDB.Close()
		}
		if ia.asnDB != nil {
			ia.asnDB.Close()
		}
	}()

	defer close(ia.asnDBChan)
	defer close(ia.cityDBChan)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		ia.errChan <- fmt.Errorf("failed to create watcher: %w", err)
		return
	}
	defer watcher.Close()

	if err := watcher.Add(ia.cityDBPath); err != nil {
		ia.errChan <- fmt.Errorf("failed to watch city database %q: %w", ia.cityDBPath, err)
		return
	}
	if err := watcher.Add(ia.asnDBPath); err != nil {
		ia.errChan <- fmt.Errorf("failed to watch asn database %q: %w", ia.asnDBPath, err)
		return
	}

	if err := ia.loadDBs(); err != nil {
		ia.errChan <- err
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case ia.asnDBChan <- ia.asnDB:
		case ia.cityDBChan <- ia.cityDB:
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Write) {
				log.Println("fs watcher modified file:", event.Name)
				if err := ia.loadDBs(); err != nil {
					ia.errChan <- err
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Println("fs watcher error:", err)
		}
	}
}

// getASNDB returns the ASN database reader, or nil if not loaded.
func (ia *MaxMindMMDBAdapter) getASNDB() *maxminddb.Reader {
	db, ok := <-ia.asnDBChan
	if !ok {
		return nil
	}
	return db
}

// getCityDB returns the City database reader, or nil if not loaded.
func (ia *MaxMindMMDBAdapter) getCityDB() *maxminddb.Reader {
	db, ok := <-ia.cityDBChan
	if !ok {
		return nil
	}
	return db
}

// GetASNRecord looks up the given address in the ASN database.
func (ia *MaxMindMMDBAdapter) GetASNRecord(addr netip.Addr) (*ASNRecord, error) {
	db := ia.getASNDB()
	if db == nil {
		return nil, fmt.Errorf("maxmind: asn database not loaded (or the adapter is likely closed)")
	}
	var rec ASNRecord
	if err := db.Lookup(addr).Decode(&rec); err != nil {
		return nil, fmt.Errorf("maxmind: asn lookup failed for %s: %w", addr, err)
	}
	return &rec, nil
}

// GetCityRecord looks up the given address in the City database.
func (ia *MaxMindMMDBAdapter) GetCityRecord(addr netip.Addr) (*CityRecord, error) {
	db := ia.getCityDB()
	if db == nil {
		return nil, fmt.Errorf("maxmind: city database not loaded")
	}
	var rec CityRecord
	if err := db.Lookup(addr).Decode(&rec); err != nil {
		return nil, fmt.Errorf("maxmind: city lookup failed for %s: %w", addr, err)
	}
	return &rec, nil
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
	asnRec, err := ia.GetASNRecord(addr)
	if err != nil {
		return nil, fmt.Errorf("maxmind: asn lookup failed for %s: %w", addr, err)
	}

	// --- City lookup ---
	cityRec, err := ia.GetCityRecord(addr)
	if err != nil {
		return nil, fmt.Errorf("maxmind: city lookup failed for %s: %w", addr, err)
	}

	if asnRec.ASN != 0 {
		basicInfo.ASN = fmt.Sprintf("AS%d", asnRec.ASN)
	}
	if asnRec.Org != "" {
		basicInfo.ISP = asnRec.Org
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
// User is required to call Run(ctx) first before using the adapter.
func NewMaxMindMMDBAdapter(name, cityMMDBFile, asnMMDBFile string) (*MaxMindMMDBAdapter, error) {
	if cityMMDBFile == "" {
		return nil, fmt.Errorf("maxmind: cityMMDBFile must be specified")
	}
	if asnMMDBFile == "" {
		return nil, fmt.Errorf("maxmind: asnMMDBFile must be specified")
	}

	adapter := &MaxMindMMDBAdapter{
		name:       name,
		cityDBPath: cityMMDBFile,
		asnDBPath:  asnMMDBFile,
		asnDBChan:  make(chan *maxminddb.Reader),
		cityDBChan: make(chan *maxminddb.Reader),
		errChan:    make(chan error, 1),
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
