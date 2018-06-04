package licensing

import (
	"strings"
	"time"
)

type LicenseClient interface {
	CheckAccountExists(username string) (bool, error)
	CreateAccount(username, password, email, otherstuff string) (HubSession, error)
	Login(username, password string) (HubSession, error)
	ListLicenses(session HubSession) ([]LicenseMetadata, error) // TODO - better return type
	ListNamespaces(session HubSession) ([]string, error)        // TODO better type
	DownloadLicense(session HubSession, namespace string, subscriptionID string) (*LicenseMetadata, error)
	CreateTrial(session HubSession, namespace string, otherstuff string) (*LicenseMetadata, error)
}

type HubSession struct {
	Token string
}

// From here down needs more refinement...
var (
	HubAPI   = "https://hub.docker.com/v2"
	StoreAPI = "https://store.docker.com/api"
)

type HubUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}
type HubLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}
type HubLoginResponse struct {
	Token string `json:"token"`
}

type EUSA struct {
	Accepted bool `json:"accepted"`
}

type LicenseRequest struct {
	DockerID          string   `json:"docker_id"`
	EUSA              EUSA     `json:"eusa"`
	Name              string   `json:"name"`
	PricingComponents []string `json:"pricing_components"`
	ProductID         string   `json:"product_id"`
	ProductRatePlan   string   `json:"product_rate_plan"`
}

// LicenseConfig is downloaded from HUB or the license server (common with DTR)
type LicenseConfig struct {
	KeyID         string `yaml:"key_id" json:"key_id"`
	PrivateKey    string `yaml:"private_key" json:"private_key,omitempty"`
	Authorization string `yaml:"authorization" json:"authorization,omitempty"`
}

// CheckLicenseResponse is used to respond to license validation requests.
// Copied from dhe-license-server/requests
type CheckLicenseResponse struct {
	Expiration      time.Time `json:"expiration"` // In RFC3339 time format
	Token           string    `json:"token"`
	MaxEngines      int       `json:"maxEngines"`
	Type            string    `json:"licenseType"`
	Tier            string    `json:"tier"`
	ScanningEnabled bool      `json:"scanningEnabled"`
}

func (l CheckLicenseResponse) IsBasic() bool {
	// TODO doesn't seem possible yet at this level...
	return false
}

type LicenseMetadata struct {
	// Note: Theres a lot more in here that we're ignoring for the moment...
	Name           string `json:"name"`
	SubscriptionID string `json:"subscription_id"`
	State          string `json:"state"` // Ignore "expired" licenses - focus on "active"
	ProductID      string `json:"product_id"`
	// TODO flesh out more of these so this is more useful if someone has a bunch of different ones
}

func (l LicenseMetadata) IsBasic() bool {
	// TODO must fix before shipping a product based on this
	return strings.Contains(strings.ToLower(l.Name), "basic") // TOTAL HACK!
}
