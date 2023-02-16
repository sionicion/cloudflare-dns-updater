package main

import (
	"fmt"
	"github.com/joho/godotenv"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type AppState struct {
	apiKey           string
	zoneId           string
	ipv4RecordId     string
	ipv6RecordId     string
	ipv4Address      string
	ipv6Address      string
	updateRequired   bool
	sleep            int
	networkInterface *net.Interface
}

func main() {
	// Load the environment variables
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Error loading .env file")
		os.Exit(2)
	}

	ipv4Address := ""
	ipv6Address := ""
	updateRequired := true

	// Set variables from environment variables
	networkInterfaceName := os.Getenv("networkInterfaceName")
	if networkInterfaceName == "" {
		panic(fmt.Errorf("no network interface specified"))
	}
	// Get the network interface object
	networkInterface, exists := net.InterfaceByName(networkInterfaceName)
	if exists != nil {
		panic(fmt.Errorf("network interface %s not found", networkInterfaceName))
	}
	apiKey := os.Getenv("apiKey")
	if apiKey == "" {
		apiKey = ""
	}
	zoneId := os.Getenv("zoneId")
	if zoneId == "" {
		zoneId = ""
	}
	ipv4RecordId := os.Getenv("ipv4RecordId")
	if ipv4RecordId == "" {
		ipv4RecordId = ""
	}
	ipv6RecordId := os.Getenv("ipv6RecordId")
	if ipv6RecordId == "" {
		ipv6RecordId = ""
	}
	sleepValue := os.Getenv("sleepTime")
	sleep := 300000
	if sleepValue != "" {
		sleepInt, err := strconv.Atoi(sleepValue)
		if err == nil {
			sleep = sleepInt
		}
	}

	// Set the initial app state
	appState := AppState{
		apiKey:           apiKey,
		zoneId:           zoneId,
		ipv4Address:      ipv4Address,
		ipv6Address:      ipv6Address,
		ipv4RecordId:     ipv4RecordId,
		ipv6RecordId:     ipv6RecordId,
		updateRequired:   updateRequired,
		sleep:            sleep,
		networkInterface: networkInterface,
	}

	// Run the application immediately and then wait for the specified amount of time
	for {
		// Run the application code once immediately
		newAppState, err := runApp(appState)
		if err != nil {
			fmt.Println(err)
			return
		}
		appState = newAppState

		// Wait for the specified amount of time before running the application code again
		select {
		case <-time.After(time.Duration(appState.sleep) * time.Millisecond):
			// Do nothing - the loop will run the application code again
		case <-time.After(time.Duration(appState.sleep + 60000) * time.Millisecond):
			// Handle the case where the application is stuck or unresponsive
			fmt.Println("Application is stuck or unresponsive")
			return
		}
	}

}

func runApp(appState AppState) (AppState, error) {
	// Get the current IP addresses
	appState, err0 := checkAddresses(appState)
	if err0 != nil {
		return appState, err0
	}

	// Check if the IP addresses have changed
	if appState.updateRequired {
		// Update the DNS records
		err1 := updateDNSRecord(appState.apiKey, appState.zoneId, appState.ipv4RecordId, appState.ipv4Address)
		if err1 != nil {
			return appState, err1
		}
		err2 := updateDNSRecord(appState.apiKey, appState.zoneId, appState.ipv6RecordId, appState.ipv6Address)
		if err2 != nil {
			return appState, err2
		}
		if err1 == nil && err2 == nil {
			appState.updateRequired = false
		}
	} else {
		fmt.Println(" No update required")
	}

	// Return nil if the application ran successfully
	return appState, nil
}

// Check the current IP addresses
func checkAddresses(appState AppState) (AppState, error) {
	// Get the current public facing IPv4 address
	response, error := http.Get("https://ifconfig.me/ip")
	if error != nil {
		return appState, error
	}
	defer response.Body.Close()
	ipv4Address, error := io.ReadAll(response.Body)
	if error != nil {
		return appState, error
	}
	if appState.ipv4Address != string(ipv4Address) {
		appState.ipv4Address = string(ipv4Address)
		appState.updateRequired = true
	}

	// Get the current IPv6 address
	addresses, error := appState.networkInterface.Addrs()
	if error != nil {
		return appState, error
	}
	for _, addresses := range addresses {
		ipnet, ok := addresses.(*net.IPNet)
		if ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() == nil && ipnet.IP.IsGlobalUnicast() && ipnet.IP.To16() != nil {
			if appState.ipv6Address != ipnet.IP.String() {
				appState.ipv6Address = ipnet.IP.String()
				appState.updateRequired = true
			}
		}
	}

	message := fmt.Sprintf("\n Current public IP addresses on interface %s: \n %s \n %s", appState.networkInterface.Name, appState.ipv4Address, appState.ipv6Address)
	fmt.Println(message)
	return appState, nil
}

// Update the DNS record
func updateDNSRecord(apiKey string, zoneId string, recordId string, ipAddress string) error {
	message := fmt.Sprintf("\n Updating DNS record %s with IP address %s \n", recordId, ipAddress)
	fmt.Println(message)

	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records/%s", zoneId, recordId)
	payload := strings.NewReader(fmt.Sprintf(`{
		"content": "%s"
	  }`, ipAddress))
	request, _ := http.NewRequest("PATCH", url, payload)
	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("Authorization", "Bearer "+apiKey)
	response, error := http.DefaultClient.Do(request)
	if error != nil {
		return (error)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	fmt.Println(response)
	fmt.Println(string(body))
	return nil
}
