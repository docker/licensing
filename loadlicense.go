package licensing

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
)

var (
	LicenseNamePrefix = "com.docker.license"
	LicenseFilename   = "docker.lic"
)

// LoadLicense will store the license on the host filesystem and swarm (if swarm is active)
func LoadLicense(ctx context.Context, clnt client.CommonAPIClient, license []byte, rootDir string) error {

	// First determine if we're in swarm-mode or a stand-alone engine
	_, err := clnt.NodeList(ctx, types.NodeListOptions{})
	if err != nil { // TODO - check for the specific error message
		return WriteLicenseToHost(ctx, clnt, license, rootDir)
	}
	// Load this in the latest license index
	latestVersion, err := GetLatestNamedConfig(clnt, LicenseNamePrefix)
	if err != nil {
		return fmt.Errorf("unable to get latest license version: %s", err)
	}
	spec := swarm.ConfigSpec{
		Annotations: swarm.Annotations{
			Name: fmt.Sprintf("%s-%d", LicenseNamePrefix, latestVersion+1),
			Labels: map[string]string{
				"com.docker.ucp.access.label":     "/",
				"com.docker.ucp.collection":       "swarm",
				"com.docker.ucp.collection.root":  "true",
				"com.docker.ucp.collection.swarm": "true",
			},
		},
		Data: license,
	}
	_, err = clnt.ConfigCreate(context.Background(), spec)
	if err != nil {

		return fmt.Errorf("Failed to create license: %s", err)
	}

	return nil
}

// getLatestNamedConfig looks for versioned instances of configs with the
// given name prefix which have a `-NUM` integer version suffix. Returns the
// config with the higest version number found or nil if no such configs exist
// along with its version number.
func GetLatestNamedConfig(dclient client.CommonAPIClient, namePrefix string) (int, error) {
	latestVersion := -1
	// List any/all existing configs so that we create a newer version than
	// any that already exist.
	filter := filters.NewArgs()
	filter.Add("name", namePrefix)
	existingConfigs, err := dclient.ConfigList(context.Background(), types.ConfigListOptions{Filters: filter})
	if err != nil {
		return latestVersion, fmt.Errorf("unable to list existing configs: %s", err)
	}

	for _, existingConfig := range existingConfigs {
		existingConfigName := existingConfig.Spec.Name
		nameSuffix := strings.TrimPrefix(existingConfigName, namePrefix)
		if nameSuffix == "" || nameSuffix[0] != '-' {
			continue // No version specifier?
		}

		versionSuffix := nameSuffix[1:] // Trim the version separator.
		existingVersion, err := strconv.Atoi(versionSuffix)
		if err != nil {
			continue // Unable to parse version as integer.
		}
		if existingVersion > latestVersion {
			latestVersion = existingVersion
		}
	}

	return latestVersion, nil
}

func WriteLicenseToHost(ctx context.Context, dclient client.CommonAPIClient, license []byte, rootDir string) error {
	// TODO we should write the file out over the clnt instead of to the local filesystem
	return ioutil.WriteFile(filepath.Join(rootDir, LicenseFilename), license, 0644)
}
