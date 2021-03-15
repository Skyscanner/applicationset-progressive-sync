package utils

import (
	"io/ioutil"
	"os"
)

type Configuration struct {
	ArgoCDAuthToken  string
	ArgoCDServerAddr string
}

func getArgoCDServerAddr() (string, error) {
	path := DefaultConfigPath + ArgoCDServerAddrFile

	file, err := os.Open(path)
	if err != nil {
		return "", err
	}

	defer file.Close()

	data, err := ioutil.ReadAll(file)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func getArgoCDAuthToken() (string, error) {
	path := DefaultConfigPath + ArgoCDAuthTokenFile

	file, err := os.Open(path)
	if err != nil {
		return "", err
	}

	defer file.Close()

	data, err := ioutil.ReadAll(file)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func ReadConfiguration() (*Configuration, error) {
	acdAuthToken, err := getArgoCDAuthToken()
	if err != nil {
		return nil, err
	}

	acdServerAddr, err := getArgoCDServerAddr()
	if err != nil {
		return nil, err
	}

	c := Configuration{
		ArgoCDAuthToken:  acdAuthToken,
		ArgoCDServerAddr: acdServerAddr,
	}

	return &c, nil
}
