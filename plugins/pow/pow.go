package main

import (
	"encoding/json"
	"errors"
	"net/http"
)

type PowConfig struct {
	Bin        string
	DstPort    int
	HTTPPort   int
	DNSPort    int
	Timeout    int
	Workers    int
	Domains    []string
	ExtDomains []string
	HostRoot   string
	LogRoot    string
	RVMPath    string
}

func loadPowConfig() (*PowConfig, error) {
	req, err := http.NewRequest("GET", "http://localhost:20559/config.json", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Host", "pow")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return nil, errors.New("unable to load pow config")
	}

	var config *PowConfig

	err = json.NewDecoder(res.Body).Decode(&config)
	if err != nil {
		return nil, err
	}

	return config, nil
}
