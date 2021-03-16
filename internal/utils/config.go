package utils

import (
	"io/ioutil"
	"os"
)

type Configuration struct {
	ArgoCDAuthToken  string
	ArgoCDServerAddr string
}

func readFromFile(path string) (string, error) {
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
	acdAuthToken, err := readFromFile(ConfigDirectory + ArgoCDServerAddrFile)
	if err != nil {
		return nil, err
	}

	acdServerAddr, err := readFromFile(ConfigDirectory + ArgoCDAuthTokenFile)
	if err != nil {
		return nil, err
	}

	c := Configuration{
		ArgoCDAuthToken:  acdAuthToken,
		ArgoCDServerAddr: acdServerAddr,
	}

	return &c, nil
}
