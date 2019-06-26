package licensing_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"testing"

	"github.com/docker/licensing"
	"github.com/stretchr/testify/require"
)

var (
	mux    *http.ServeMux
	server *httptest.Server
	client licensing.Client
)

const (
	testDockerID  = "testDockerID"
	testAuthToken = "testAuthToken"
)

func setup() func() {
	mux = http.NewServeMux()
	server = httptest.NewServer(mux)

	parsedURL, err := url.Parse(server.URL)
	if err != nil {
		log.Fatal(err)
	}

	client, err = licensing.New(&licensing.Config{
		BaseURI:    *parsedURL,
		PublicKeys: []string{"LS0tLS1CRUdJTiBQVUJMSUMgS0VZLS0tLS0Ka2lkOiBKN0xEOjY3VlI6TDVIWjpVN0JBOjJPNEc6NEFMMzpPRjJOOkpIR0I6RUZUSDo1Q1ZROk1GRU86QUVJVAoKTUlJQ0lqQU5CZ2txaGtpRzl3MEJBUUVGQUFPQ0FnOEFNSUlDQ2dLQ0FnRUF5ZEl5K2xVN283UGNlWSs0K3MrQwpRNU9FZ0N5RjhDeEljUUlXdUs4NHBJaVpjaVk2NzMweUNZbndMU0tUbHcrVTZVQy9RUmVXUmlvTU5ORTVEczVUCllFWGJHRzZvbG0ycWRXYkJ3Y0NnKzJVVUgvT2NCOVd1UDZnUlBIcE1GTXN4RHpXd3ZheThKVXVIZ1lVTFVwbTEKSXYrbXE3bHA1blEvUnhyVDBLWlJBUVRZTEVNRWZHd20zaE1PL2dlTFBTK2hnS1B0SUhsa2c2L1djb3hUR29LUAo3OWQvd2FIWXhHTmw3V2hTbmVpQlN4YnBiUUFLazIxbGc3OThYYjd2WnlFQVRETXJSUjlNZUU2QWRqNUhKcFkzCkNveVJBUENtYUtHUkNLNHVvWlNvSXUwaEZWbEtVUHliYncwMDBHTyt3YTJLTjhVd2dJSW0waTVJMXVXOUdrcTQKempCeTV6aGdxdVVYYkc5YldQQU9ZcnE1UWE4MUR4R2NCbEp5SFlBcCtERFBFOVRHZzR6WW1YakpueFpxSEVkdQpHcWRldlo4WE1JMHVrZmtHSUkxNHdVT2lNSUlJclhsRWNCZi80Nkk4Z1FXRHp4eWNaZS9KR1grTEF1YXlYcnlyClVGZWhWTlVkWlVsOXdYTmFKQitrYUNxejVRd2FSOTNzR3crUVNmdEQwTnZMZTdDeU9IK0U2dmc2U3QvTmVUdmcKdjhZbmhDaVhJbFo4SE9mSXdOZTd0RUYvVWN6NU9iUHlrbTN0eWxyTlVqdDBWeUFtdHRhY1ZJMmlHaWhjVVBybQprNGxWSVo3VkQvTFNXK2k3eW9TdXJ0cHNQWGNlMnBLRElvMzBsSkdoTy8zS1VtbDJTVVpDcXpKMXlFbUtweXNICjVIRFc5Y3NJRkNBM2RlQWpmWlV2TjdVQ0F3RUFBUT09Ci0tLS0tRU5EIFBVQkxJQyBLRVktLS0tLQo="},
	})
	if err != nil {
		log.Fatal(err)
	}

	return func() {
		server.Close()
	}
}

func fixture(path string) string {
	b, err := ioutil.ReadFile("testdata/fixtures/" + path)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func TestClient_GenerateNewTrialSubscription(t *testing.T) {
	teardown := setup()
	defer teardown()

	ctx := context.Background()

	// return account
	mux.HandleFunc(path.Join("/api/billing/v4/accounts", testDockerID), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, fixture("account.json"))
	})

	// return new subscription
	mux.HandleFunc("/api/billing/v4/subscriptions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, fixture("subscription.json"))
	})

	subID, err := client.GenerateNewTrialSubscription(ctx, testAuthToken, testDockerID)
	require.NoError(t, err)
	require.Equal(t, "testSubscriptionID", subID)
}

func TestClient_ListSubscriptions(t *testing.T) {
	teardown := setup()
	defer teardown()

	ctx := context.Background()

	// return list of subscriptions
	mux.HandleFunc("/api/billing/v4/subscriptions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, fixture("subscriptions.json"))
	})

	subs, err := client.ListSubscriptions(ctx, testAuthToken, testDockerID)
	require.NoError(t, err)
	// test filter, should only be one sub with 'docker-ee' prefix
	require.Len(t, subs, 1)
}

func TestClient_VerifyLicense(t *testing.T) {
	teardown := setup()
	defer teardown()

	ctx := context.Background()

	licBytes, err := ioutil.ReadFile("testdata/test-license.lic")
	require.NoError(t, err)

	lic, err := client.ParseLicense(licBytes)
	require.NoError(t, err)

	_, err = client.VerifyLicense(ctx, *lic)
	require.NoError(t, err)
}

func TestClient_SummarizeLicense(t *testing.T) {
	teardown := setup()
	defer teardown()

	ctx := context.Background()

	licBytes, err := ioutil.ReadFile("testdata/expired-license.lic")
	require.NoError(t, err)

	lic, err := client.ParseLicense(licBytes)
	require.NoError(t, err)

	cr, err := client.VerifyLicense(ctx, *lic)
	require.NoError(t, err)

	summary := client.SummarizeLicense(cr).String()
	expected := "Components: 1 Nodes	Expiration date: 2018-03-18	Expired! You will no longer receive updates. Please renew at https://docker.com/licensing"
	require.Equal(t, summary, expected)
}
