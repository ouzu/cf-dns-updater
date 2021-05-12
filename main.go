package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/cloudflare/cloudflare-go"
)

const reqStr = "<?xml version='1.0' encoding='utf-8'?> <s:Envelope s:encodingStyle='http://schemas.xmlsoap.org/soap/encoding/' xmlns:s='http://schemas.xmlsoap.org/soap/envelope/'> <s:Body> <u:GetExternalIPAddress xmlns:u='urn:schemas-upnp-org:service:WANIPConnection:1' /> </s:Body> </s:Envelope>"

func main() {

	fbAddr := flag.String("fb", "192.168.178.1", "FritzBox address")
	cfMail := flag.String("email", "", "Cloudflare email address")
	cfKey := flag.String("apikey", "", "Cloudflare API key")
	cfKeyPath := flag.String("apikeypath", "", "path to API key file")
	flag.Parse()

	domains := flag.Args()

	if len(domains) == 0 {
		fmt.Println("No domains given")
		return
	}

	if *cfMail == "" {
		log.Fatalln("No Cloudflare email address given")
	}

	if *cfKey == "" && *cfKeyPath == "" {
		log.Fatalln("No API key or key file given")
	}

	if *cfKey != "" && *cfKeyPath != "" {
		log.Fatalln("Both API key or key file given")
	}

	if *cfKey == "" {
		file, err := ioutil.ReadFile(*cfKeyPath)
		if err != nil {
			log.Fatalln(err)
		}

		*cfKey = string(file)
	}

	cfClient, err := cloudflare.New(*cfKey, *cfMail)
	if err != nil {
		log.Fatalln(err)
	}

	client := &http.Client{}
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s:49000/igdupnp/control/WANIPConn1", *fbAddr), strings.NewReader(reqStr))
	req.Header.Set("Content-Type", "text/xml; charset=\"utf-8\"")
	req.Header.Set("SoapAction", "urn:schemas-upnp-org:service:WANIPConnection:1#GetExternalIPAddress")

	res, err := client.Do(req)
	if err != nil {
		log.Fatalln(err)
	}

	if res.StatusCode != http.StatusOK {
		log.Fatalf("FritzBox returned status %s\n", res.Status)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Fatalln(err)
	}

	ipRe := regexp.MustCompile("\\b(?:[0-9]{1,3}\\.){3}[0-9]{1,3}\\b")

	ipMatches := ipRe.FindStringSubmatch(string(body))

	if len(ipMatches) == 0 {
		log.Fatalln("FritzBox returned no address")
	}

	ipAddress := ipMatches[0]

	log.Println("External address is", ipAddress)

	for _, domain := range domains {
		r := &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{
					Timeout: time.Millisecond * time.Duration(10000),
				}
				return d.DialContext(ctx, network, "1.1.1.1:53")
			},
		}
		ip, _ := r.LookupHost(context.Background(), domain)

		if ip[0] == ipAddress {
			log.Println(domain, "is up to date")
			continue
		}

		log.Println("Updating", domain)
		id, err := cfClient.ZoneIDByName(domain)
		if err != nil {
			log.Fatalln(err)
		}

		records, err := cfClient.DNSRecords(context.Background(), id, cloudflare.DNSRecord{})

		updated := false

		for _, record := range records {
			if record.Name == domain && record.Type == "A" {
				if record.Content == ipAddress {
					log.Println("Record is up to date")
					continue
				}
				log.Println("Updating record")
				cfClient.UpdateDNSRecord(context.Background(), id, record.ID, cloudflare.DNSRecord{
					Content: ipAddress,
				})
				updated = true
			}
		}

		if !updated {
			cfClient.CreateDNSRecord(context.Background(), id, cloudflare.DNSRecord{
				Name:    domain,
				Content: ipAddress,
			})
		}
	}
}
