package main

import (
	"encoding/json"
	"log"
	"net/netip"
	"os"

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

type ASNRecord struct {
	ASN uint32 `maxminddb:"autonomous_system_number"`
	Org string `maxminddb:"autonomous_system_organization"`
}

func parseASN() {
	db, err := maxminddb.Open("GeoLite2-ASN.mmdb")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	ip, err := netip.ParseAddr("81.2.69.142")
	if err != nil {
		log.Fatal(err)
	}

	var record ASNRecord
	err = db.Lookup(ip).Decode(&record)
	if err != nil {
		log.Fatal(err)
	}

	json.NewEncoder(os.Stdout).Encode(record)
}

func main() {
	db, err := maxminddb.Open("GeoLite2-City.mmdb")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	ip, err := netip.ParseAddr("81.2.69.142")
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Querying City ...")
	var record CityRecord
	err = db.Lookup(ip).Decode(&record)
	if err != nil {
		log.Fatal(err)
	}

	json.NewEncoder(os.Stdout).Encode(record)

	log.Println("Querying ASN ...")
	parseASN()
}
