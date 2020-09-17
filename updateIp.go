package main

import (
	"github.com/cloudflare/cloudflare-go"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
)

const (
	CloudflareKey = "CloudflareKey"

	ZoneID = "ZoneID"

	V4RecordID = "V4RecordID"
	V6RecordID = "V6RecordID"
)

var API *cloudflare.API
var err error

func updateDNSV4(ip string) error {
	err := API.UpdateDNSRecord(ZoneID, V4RecordID, cloudflare.DNSRecord{
		Type: "A",
		Name: "_autov4.DOMAIN",
		Content: ip,
	})
	if err != nil {
		log.Printf("update dns record failed: (v4) %v\n", err)
		return err
	}
	return nil
}

func updateDNSV6(ip string) error {
	err = API.UpdateDNSRecord(ZoneID, V6RecordID, cloudflare.DNSRecord{
		Type: "AAAA",
		Name: "_autov6.DOMAIN",
		Content: ip,
	})
	if err != nil {
		log.Printf("update dns record failed: (v6) %v\n", err)
		return err
	}
	return nil
}

func get(u string) string {
	r, err := http.Get(u)
	if err != nil {
		log.Printf("get ip failed: (v4) %v", err)
		return ""
	}
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("parse response body error: %v", err)
		return ""
	}
	return string(data)
}

func main() {
	API, err = cloudflare.NewWithAPIToken(CloudflareKey)
	if err != nil {
		log.Fatalf("cloudflare failed: (initialization failure) %v", err)
	}

	ipv4 := get("https://api.ipify.org/")
	ipv6 := get("https://api6.ipify.org/")

	if err = updateDNSV4(ipv4); err != nil {
		log.Fatalf("cloudflare failed: (update v4) %v", err)
	}

	if strings.Count(ipv6, ":") >= 2 { // is true IPv6
		if err = updateDNSV6(ipv6); err != nil {
			log.Fatalf("cloudflare failed: (update v6) %v", err)
		}
		log.Printf("successfully updated ipv6 to %s\n", ipv6)
	} else {
		log.Printf("not updating ipv6 to %s because of invalid address\n", ipv6)
	}

	log.Printf("successfully updated ipv4 to %s\n", ipv4)
}