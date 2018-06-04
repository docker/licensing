package licensing

// TODO this should be vendored from someplace else

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/docker/libtrust"
)

// ParseLicensePayload will parse a license file and verify it is valid
func ParseLicensePayload(data []byte) (*CheckLicenseResponse, error) {
	license := new(LicenseConfig)
	err := json.Unmarshal(data, &license)
	if err != nil {
		return nil, err
	}

	authorizationBytes, err := base64.StdEncoding.DecodeString(license.Authorization)
	if err != nil {
		return nil, fmt.Errorf("Malformed license key - Failed to decode key authorization: %s", err)
	}

	// Process the new license we just got
	jsonSignature, err := libtrust.ParseJWS(authorizationBytes)
	if err != nil {
		return nil, fmt.Errorf("Malformed renewal from license server: %s", err)
	}
	publicKey, err := getLicenseServerPublicKey()
	if err != nil {
		return nil, fmt.Errorf("Malformed license server public key: %s", err)
	}
	checkResponse, err := verifyJSONSignature(license.PrivateKey, jsonSignature, publicKey)
	if err != nil {
		return nil, fmt.Errorf("Malformed license key - Signature verification failure: %s", err)
	}
	return checkResponse, nil
}

// Official production license server public key
var DefaultLicenseServerPublicKey = "LS0tLS1CRUdJTiBQVUJMSUMgS0VZLS0tLS0Ka2lkOiBKN0xEOjY3VlI6TDVIWjpVN0JBOjJPNEc6NEFMMzpPRjJOOkpIR0I6RUZUSDo1Q1ZROk1GRU86QUVJVAoKTUlJQ0lqQU5CZ2txaGtpRzl3MEJBUUVGQUFPQ0FnOEFNSUlDQ2dLQ0FnRUF5ZEl5K2xVN283UGNlWSs0K3MrQwpRNU9FZ0N5RjhDeEljUUlXdUs4NHBJaVpjaVk2NzMweUNZbndMU0tUbHcrVTZVQy9RUmVXUmlvTU5ORTVEczVUCllFWGJHRzZvbG0ycWRXYkJ3Y0NnKzJVVUgvT2NCOVd1UDZnUlBIcE1GTXN4RHpXd3ZheThKVXVIZ1lVTFVwbTEKSXYrbXE3bHA1blEvUnhyVDBLWlJBUVRZTEVNRWZHd20zaE1PL2dlTFBTK2hnS1B0SUhsa2c2L1djb3hUR29LUAo3OWQvd2FIWXhHTmw3V2hTbmVpQlN4YnBiUUFLazIxbGc3OThYYjd2WnlFQVRETXJSUjlNZUU2QWRqNUhKcFkzCkNveVJBUENtYUtHUkNLNHVvWlNvSXUwaEZWbEtVUHliYncwMDBHTyt3YTJLTjhVd2dJSW0waTVJMXVXOUdrcTQKempCeTV6aGdxdVVYYkc5YldQQU9ZcnE1UWE4MUR4R2NCbEp5SFlBcCtERFBFOVRHZzR6WW1YakpueFpxSEVkdQpHcWRldlo4WE1JMHVrZmtHSUkxNHdVT2lNSUlJclhsRWNCZi80Nkk4Z1FXRHp4eWNaZS9KR1grTEF1YXlYcnlyClVGZWhWTlVkWlVsOXdYTmFKQitrYUNxejVRd2FSOTNzR3crUVNmdEQwTnZMZTdDeU9IK0U2dmc2U3QvTmVUdmcKdjhZbmhDaVhJbFo4SE9mSXdOZTd0RUYvVWN6NU9iUHlrbTN0eWxyTlVqdDBWeUFtdHRhY1ZJMmlHaWhjVVBybQprNGxWSVo3VkQvTFNXK2k3eW9TdXJ0cHNQWGNlMnBLRElvMzBsSkdoTy8zS1VtbDJTVVpDcXpKMXlFbUtweXNICjVIRFc5Y3NJRkNBM2RlQWpmWlV2TjdVQ0F3RUFBUT09Ci0tLS0tRU5EIFBVQkxJQyBLRVktLS0tLQo="

func getLicenseServerPublicKey() (libtrust.PublicKey, error) {
	pemBytes, err := base64.StdEncoding.DecodeString(DefaultLicenseServerPublicKey)
	if err != nil {
		return nil, fmt.Errorf("Failed to decode the embedded license public key during init: %s", err)
	}

	publicKey, err := libtrust.UnmarshalPublicKeyPEM(pemBytes)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse the embedded license public key during init: %s", err)
	}
	return publicKey, err
}

// Copied ~verbatim from DTR so we don't have to vendor in a mountain of unnecessary dependencies
func verifyJSONSignature(privateKey string, jsonSignature *libtrust.JSONSignature, publicKey libtrust.PublicKey) (*CheckLicenseResponse, error) {
	keys, err := jsonSignature.Verify()
	if err != nil {
		return nil, err
	} else if len(keys) != 1 || keys[0].KeyID() != publicKey.KeyID() {
		return nil, errors.New("Bad signature")
	}

	payload, err := jsonSignature.Payload()
	if err != nil {
		return nil, err
	}

	fmt.Printf("XXX license payload %s\n", string(payload))

	checkLicenseResponse := new(CheckLicenseResponse)
	if err := json.NewDecoder(bytes.NewReader(payload)).Decode(checkLicenseResponse); err != nil {
		return nil, err
	}

	ok, err := CheckToken(checkLicenseResponse.Expiration.Format(time.RFC3339), checkLicenseResponse.Token, privateKey)
	if err != nil {
		return nil, err
	} else if !ok {
		return nil, errors.New("Invalid token")
	}

	return checkLicenseResponse, nil
}

// CheckToken validates the given token and private key.
// Copied from dhe-license-server/verify
func CheckToken(message, token, privateKey string) (bool, error) {
	tokenBytes, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		return false, err
	}

	generatedToken, err := GenerateToken(message, privateKey)
	if err != nil {
		return false, err
	}

	generatedBytes, err := base64.URLEncoding.DecodeString(generatedToken)
	if err != nil {
		return false, err
	}

	return hmac.Equal(tokenBytes, generatedBytes), nil
}

// GenerateToken creates a token from the given private key.
// Copied from dhe-license-server/verify
func GenerateToken(message, privateKey string) (string, error) {
	key, err := base64.URLEncoding.DecodeString(privateKey)
	if err != nil {
		return "", err
	}

	h := hmac.New(sha256.New, key)
	h.Write([]byte(message))
	return base64.URLEncoding.EncodeToString(h.Sum(nil)), nil
}

// GetTokenHeader takes a token and builds a proper header string
func GetTokenHeader(token string) (string, string) {
	return "Authorization", fmt.Sprintf("Bearer %s", token)
}
