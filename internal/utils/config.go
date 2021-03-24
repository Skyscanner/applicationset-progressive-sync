package utils

import (
	"io/ioutil"
	"os"
)

// Configuration holds the configuration to connect to the Argo CD server
type Configuration struct {
	ArgoCDAuthToken  string
	ArgoCDServerAddr string
}

// readFromFile returns the content of a file
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

// readFromEnvOrFile returns the value of an environment variable if present, or from a file otherwise
func readFromEnvOrFile(secretName string) (string, error) {
	var err error
	value := os.Getenv(secretName)

	if len(value) == 0 {
		value, err = readFromFile(ConfigDirectory + secretName)
		if err != nil {
			return "", err
		}
	}

	return value, nil
}

/*
ReadConfiguration returns a Configuration, reading it from environment variables if present, or from files in
the configuration directory otherwise
*/
func ReadConfiguration() (*Configuration, error) {
	var err error
	var acdAuthToken, acdServerAddr string

	acdAuthToken, err = readFromEnvOrFile(ArgoCDAuthTokenKey)
	if err != nil {
		return nil, err
	}

	acdServerAddr, err = readFromEnvOrFile(ArgoCDServerAddrKey)
	if err != nil {
		return nil, err
	}

	c := Configuration{
		ArgoCDAuthToken:  acdAuthToken,
		ArgoCDServerAddr: acdServerAddr,
	}

	return &c, nil
}
