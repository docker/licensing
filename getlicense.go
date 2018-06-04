package licensing

import (
	"bufio"
	//"encoding/base64"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	//"github.com/sirupsen/logrus"
	"golang.org/x/net/context"

	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/trust"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	registrytypes "github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/pkg/term"
	"github.com/pkg/errors"
)

func GetHubUser(httpclient *http.Client, username string) (*HubUser, error) {
	// Check to see if the account exists... if not, allow creation of a new account
	requestURL, err := url.Parse(HubAPI + "/users/" + username + "/")
	if err != nil {
		return nil, fmt.Errorf("Invalid username: %s", err)
	}
	req, err := http.NewRequest("GET", requestURL.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpclient.Do(req)
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var user HubUser
	resp.Body.Close()
	if resp.StatusCode == 200 {
		err := json.Unmarshal(body, &user)
		if err != nil {
			return nil, err
		}
		return &user, nil
	} else if resp.StatusCode == 404 {
		fmt.Printf("Username %s does not exist.  Would you like to create it now?", username)
		return nil, fmt.Errorf("Not yet implemented - creation of new hub user.")
		// TODO create a new user now...
	}
	return nil, fmt.Errorf("Unexpected error from hub: %d %s", resp.StatusCode, string(body))
}

func LoginHubUser(httpclient *http.Client, username, password string) (string, error) {
	auth := HubLoginRequest{
		Username: username,
		Password: password,
	}
	data, err := json.Marshal(auth)
	if err != nil {
		return "", err
	}
	// Check to see if the account exists... if not, allow creation of a new account
	req, err := http.NewRequest("POST", HubAPI+"/users/login", bytes.NewBuffer(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpclient.Do(req)
	if err != nil {
		return "", err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var token HubLoginResponse
	resp.Body.Close()
	if resp.StatusCode == 200 {
		err := json.Unmarshal(body, &token)
		if err != nil {
			return "", err
		}
		return token.Token, nil
	} else if resp.StatusCode == 401 {
		fmt.Printf("Username %s does not exist.  Would you like to create it now?", username)
		return "", fmt.Errorf("Not yet implemented - creation of new hub user.")
		// TODO create a new user now...
	}
	return "", fmt.Errorf("Unexpected error from hub: %d %s", resp.StatusCode, string(body))
}

func LoginViaAuth(httpclient *http.Client, authConfig *types.AuthConfig) (string, error) {
	auth := HubLoginRequest{
		Username: authConfig.Username,
		Password: authConfig.Password,
	}
	data, err := json.Marshal(auth)
	if err != nil {
		return "", err
	}
	// Check to see if the account exists... if not, allow creation of a new account
	req, err := http.NewRequest("POST", HubAPI+"/users/login", bytes.NewBuffer(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpclient.Do(req)
	if err != nil {
		return "", err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var token HubLoginResponse
	resp.Body.Close()
	if resp.StatusCode == 200 {
		err := json.Unmarshal(body, &token)
		if err != nil {
			return "", err
		}
		return token.Token, nil
	}
	return "", fmt.Errorf("Unexpected error from hub: %d %s", resp.StatusCode, string(body))
}

func GetExistingLicenses(httpclient *http.Client, token, id string) ([]LicenseMetadata, error) {

	// TODO - use https://hub.docker.com/v2/repositories/namespaces/ to determine if the user
	// can see multiple namespaces, and query all of them, then show what license maps to what namespace

	requestURL, err := url.Parse(StoreAPI + "/billing/v4/subscriptions/?docker_id=" + id)
	if err != nil {
		return nil, fmt.Errorf("Unable to generate subscription request: %s", err)
	}
	req, err := http.NewRequest("GET", requestURL.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set(GetTokenHeader(token))
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpclient.Do(req)
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var fullSet []LicenseMetadata
	var licenses []LicenseMetadata
	resp.Body.Close()
	if resp.StatusCode == 200 {
		err := json.Unmarshal(body, &fullSet)
		if err != nil {
			return nil, err
		}
		for _, licenseInfo := range fullSet {
			// Filter out expired licenses
			if licenseInfo.State == "expired" {
				continue
			}
			// Filter out licenses that aren't `docker-ee*`
			if !strings.HasPrefix(licenseInfo.ProductID, "docker-ee") {
				continue
			}
			// Looks legit, add it to the list
			licenses = append(licenses, licenseInfo)

		}
		return licenses, nil
	}
	return nil, fmt.Errorf("Unexpected error from hub: %d %s", resp.StatusCode, string(body))
}

func GenerateNewTrialLicense(httpclient *http.Client, token, id, trialName string, basic bool) ([]byte, bool, error) {

	// TODO - WARNING - it seems hub/store will currently let you generate as many trial
	//                  licenses as you want.  We might want to lock that down on the server,
	//                  but in any case, we probably want to add logic here to detect if
	//                  if there's an existing trial (expired or not) and not let you generate
	//                  a new one.
	//
	//                  Perhaps the one wrinkle is what to do if the trial has expired but
	//                  they don't have another valid non-expired license?
	requestURL, err := url.Parse(StoreAPI + "/billing/v4/subscriptions/?docker_id=" + id)
	if err != nil {
		return nil, false, fmt.Errorf("Unable to generate subscription download request: %s", err)
	}
	productID := "docker-ee-trial"
	/* XXX this doesn't work - store blocks the product ID as invalid
	if basic {
		productID = "docker-ee-server-trial"
	}
	*/
	licenseRequest := LicenseRequest{
		DockerID: id,
		EUSA: EUSA{
			Accepted: true,
		},
		Name:            trialName,
		ProductID:       productID,
		ProductRatePlan: "free-trial",
	}

	data, err := json.Marshal(licenseRequest)
	if err != nil {
		return nil, false, err
	}

	req, err := http.NewRequest("POST", requestURL.String(), bytes.NewBuffer(data))
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(GetTokenHeader(token))
	resp, err := httpclient.Do(req)
	if err != nil {
		return nil, false, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, false, err
	}
	resp.Body.Close()
	var licenseInfo LicenseMetadata
	if resp.StatusCode != 201 {
		return nil, false, fmt.Errorf("Unexpected error from hub: %d %s", resp.StatusCode, string(body))
	}
	err = json.Unmarshal(body, &licenseInfo)
	if err != nil {
		return nil, false, err
	}

	// Now retrieve the license itself
	data, _, err = DownloadLicenseFromHub(httpclient, token, id, licenseInfo.SubscriptionID)
	return data, licenseInfo.IsBasic(), err
}

func DownloadLicenseFromHub(httpclient *http.Client, token, id, subscriptionID string) ([]byte, *LicenseMetadata, error) {
	// First download the license itself
	requestURL, err := url.Parse(StoreAPI + "/billing/v4/subscriptions/" + subscriptionID + "/license-file/")
	if err != nil {
		return nil, nil, fmt.Errorf("Unable to generate subscription download request: %s", err)
	}
	req, err := http.NewRequest("GET", requestURL.String(), nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set(GetTokenHeader(token))
	resp, err := httpclient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, nil, fmt.Errorf("Unexpected error from hub: %d %s", resp.StatusCode, string(body))
	}

	// Now download the metadata too, since we can't tell basic from std/advanced in the payload (yet)
	// TODO - this is a bug in the license server/structure
	requestURL, err = url.Parse(StoreAPI + "/billing/v4/subscriptions/" + subscriptionID) // XXX trailing slash?
	if err != nil {
		return nil, nil, fmt.Errorf("Unable download license metadata: %s", err)
	}
	req, err = http.NewRequest("GET", requestURL.String(), nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set(GetTokenHeader(token))
	resp, err = httpclient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, nil, fmt.Errorf("Unexpected error from hub on license metadata: %d %s", resp.StatusCode, string(body))
	}

	var licenseInfo LicenseMetadata
	err = json.Unmarshal(body, &licenseInfo)
	if err != nil {
		return nil, nil, err
	}
	return body, &licenseInfo, nil

}

// DownloadLicense returns the license payload, true if basic, and any error if there was a problem
func DownloadLicense(ctx context.Context, cli command.Cli) ([]byte, bool, error) {
	// On Windows, force the use of the regular OS stdin stream. Fixes #14336/#14210
	/* Can't do this out of the command package...
	if runtime.GOOS == "windows" {
		cli.in = NewInStream(os.Stdin)
	}
	*/

	if !cli.In().IsTerminal() {
		return nil, false, errors.Errorf("Error: Cannot perform an interactive login from a non TTY device")
	}
	httpclient := &http.Client{} // TODO - any customization?

	// Try using the auth credential cache first
	authConfig, err := getRegistryAuth(cli)
	if err != nil {
		// TODO - this might fail if we haven't done a login - better error message maybe?
		return nil, false, fmt.Errorf("Failed to get authConfig: %s", err)
	}
	fmt.Fprintf(cli.Out(), "\nChecking Docker Store for licenses for %s... ", authConfig.Username)
	//fmt.Printf("XXX AuthConfig: %#v\n", *authConfig)
	token, err := LoginViaAuth(httpclient, authConfig)
	user, err := GetHubUser(httpclient, authConfig.Username)
	if err != nil {
		//logrus.Debug("XXX Failed to get hub user")
		return nil, false, err
	}

	if err != nil {

		// TODO - handle scenario of creating new account

		promptWithDefault(cli.Out(), "Docker Hub/Store username", "")
		username := readInput(cli.In(), cli.Out())
		username = strings.TrimSpace(username)

		user, err = GetHubUser(httpclient, username)
		if err != nil {
			return nil, false, err
		}

		// TODO - consider 3 retries if the user gets the password wrong

		// Now that we know the account exists, get the password
		oldState, err := term.SaveState(cli.In().FD())
		if err != nil {
			return nil, false, err
		}
		fmt.Fprintf(cli.Out(), "Docker Hub/Store Password: ")
		term.DisableEcho(cli.In().FD(), oldState)
		password := readInput(cli.In(), cli.Out())
		fmt.Fprint(cli.Out(), "\n")

		term.RestoreTerminal(cli.In().FD(), oldState)
		if password == "" {
			return nil, false, errors.Errorf("Error: Password Required")
		}
		token, err = LoginHubUser(httpclient, username, password)
	}

	// Check for existing licenses, allow user to pick which one to apply
	licenses, err := GetExistingLicenses(httpclient, token, user.ID)
	if err != nil {
		//logrus.Debug("XXX Failed to get existing licenses")
		return nil, false, err
	}
	pick := 0
	if len(licenses) > 0 {
		fmt.Fprintln(cli.Out(), "Licenses found.")
	}

	// DEBUG ONLY!  IRL, you shouldn't be able to generate a second trial if you already have licenses
	fmt.Fprintf(cli.Out(), "%d) Generate a new Enterprise Basic trial license (up to 10 individual engines, no orchestration)\n", 0)
	fmt.Fprintf(cli.Out(), "%d) Generate a new Enterprise Advanced trial license (up to 10 engines with orchestration)\n", 1)

	if len(licenses) > 0 {
		// TODO - should we sort this based on some critera?  (perhaps active first?)
		for i, licenseInfo := range licenses {
			// TODO - should we expose more details?  Node counts, etc?
			fmt.Fprintf(cli.Out(), "%d) %s (%s) %s\n", i+2, licenseInfo.State, licenseInfo.ProductID, licenseInfo.Name)
		}

	}
	// TODO - probably should try to be smarter about which license to default to
	//        instead of generating a new trial when they have existing licenses
	promptWithDefault(cli.Out(), "Please pick the license above by number", "0")
	pickstr := readInput(cli.In(), cli.Out())
	pickstr = strings.TrimSpace(pickstr)
	pick, err = strconv.Atoi(pickstr)
	if err != nil {
		return nil, false, err
	}
	if pick == 0 {
		trialName := fmt.Sprintf("Enterprise Basic Trial")
		//logrus.Debug("XXX generating basic trial")
		return GenerateNewTrialLicense(httpclient, token, user.ID, trialName, true)
	} else if pick == 1 {
		clnt := cli.Client()
		info, err := clnt.Info(ctx)
		if err != nil {
			return nil, false, err
		}
		trialName := fmt.Sprintf("Enterprise Advanced Trial generated for cluster %s - %s", info.Swarm.Cluster.ID, info.Swarm.NodeAddr)
		//logrus.Debug("XXX generating advanced trial")
		return GenerateNewTrialLicense(httpclient, token, user.ID, trialName, false)
	} else {
		pick -= 2
		if pick >= len(licenses) {
			return nil, false, fmt.Errorf("You must pick a valid license number from the list above.")
		}
		//logrus.Debug("XXX downloading existing license")
		data, info, err := DownloadLicenseFromHub(httpclient, token, user.ID, licenses[pick].SubscriptionID)
		return data, info.IsBasic(), err
	}
}

// XXX Yuck - cut-and-paste from ../registry.go

func promptWithDefault(out io.Writer, prompt string, configDefault string) {
	if configDefault == "" {
		fmt.Fprintf(out, "%s: ", prompt)
	} else {
		fmt.Fprintf(out, "%s (%s): ", prompt, configDefault)
	}
}

// XXX Yuck - cut-and-paste from ../registry.go

func readInput(in io.Reader, out io.Writer) string {
	reader := bufio.NewReader(in)
	line, _, err := reader.ReadLine()
	if err != nil {
		fmt.Fprintln(out, err.Error())
		os.Exit(1)
	}
	return string(line)
}

// TODO move this to a helper
func getRegistryAuth(cli command.Cli) (*types.AuthConfig, error) {
	fullName := "docker.io/dockereng/ee-engine" // XXX is this right/necessary?
	distributionRef, err := reference.ParseNormalizedNamed(fullName)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse image name: %s: %s", fullName, err)
	}
	imgRefAndAuth, err := trust.GetImageReferencesAndAuth(context.Background(), nil, authResolver(cli), distributionRef.String())
	if err != nil {
		return nil, fmt.Errorf("Failed to get imgRefAndAuth: %s", err)
	}
	return imgRefAndAuth.AuthConfig(), nil
}

func authResolver(cli command.Cli) func(ctx context.Context, index *registrytypes.IndexInfo) types.AuthConfig {
	return func(ctx context.Context, index *registrytypes.IndexInfo) types.AuthConfig {
		return command.ResolveAuthConfig(ctx, cli, index)
	}
}
