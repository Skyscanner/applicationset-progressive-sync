package utils

import (
	"io/ioutil"
	"os"
	"strconv"

	"github.com/Skyscanner/applicationset-progressive-sync/internal/consts"
)

// Configuration holds the configuration to connect to the Argo CD server
type Configuration struct {
	ArgoCDAuthToken  string
	ArgoCDServerAddr string
	ArgoCDInsecure   bool
	ArgoCDPlaintext  bool
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
func readFromEnvOrFile(paramName string) (string, error) {
	var err error
	value, ok := os.LookupEnv(paramName)

	if !ok {
		value, err = readFromFile(consts.ConfigDirectory + paramName)
		if err != nil {
			return "", err
		}
	}

	return value, nil
}

// isFlagSet returns the boolean value of an environment variable if present, or from a file otherwise
func isFlagSet(paramName string) (bool, error) {
	strValue, err := readFromEnvOrFile(paramName)
	if err != nil {
		return false, err
	}

	value, err := strconv.ParseBool(strValue)
	if err != nil {
		return false, err
	}

	return value, nil
}

// ReadConfiguration returns a Configuration, reading it from environment variables if present,
// or from files in the configuration directory otherwise
func ReadConfiguration() (Configuration, error) {
	var err error
	var acdAuthToken, acdServerAddr string
	var acdInsecure, acdPlaintext bool

	acdAuthToken, err = readFromEnvOrFile(consts.ArgoCDAuthTokenKey)
	if err != nil {
		return Configuration{}, err
	}

	acdServerAddr, err = readFromEnvOrFile(consts.ArgoCDServerAddrKey)
	if err != nil {
		return Configuration{}, err
	}

	acdInsecure, err = isFlagSet(consts.ArgoCDInsecureKey)
	if err != nil {
		return Configuration{}, err
	}

	acdPlaintext, err = isFlagSet(consts.ArgoCDPlaintextKey)
	if err != nil {
		return Configuration{}, err
	}

	c := Configuration{
		ArgoCDAuthToken:  acdAuthToken,
		ArgoCDServerAddr: acdServerAddr,
		ArgoCDInsecure:   acdInsecure,
		ArgoCDPlaintext:  acdPlaintext,
	}

	return c, nil
}
