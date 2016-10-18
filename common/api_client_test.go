package common

import (
	"testing"
	"github.com/stretchr/testify/suite"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including assertion methods.
type ApiClientTestSuite struct {
	suite.Suite
	config Config
	apiClient *ApiClient
}

func (suite *ApiClientTestSuite) SetupTest() {
	suite.config.Webservice.Endpoint = "https://testy-test.hu"
	suite.apiClient, _ = NewApiClientWithConfig(suite.config)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestApiClientTestSuite(t *testing.T) {
	suite.Run(t, new(ApiClientTestSuite))
}

func (suite *ApiClientTestSuite) TestMakeApiUrl_withPrefix() {
	url := suite.apiClient.makeApiUrl("/api/v1/wow")
	suite.Equal(url, "https://testy-test.hu/api/v1/wow")
}

func (suite *ApiClientTestSuite) TestMakeApiUrl_withPrefix_newApiVersion() {
	url := suite.apiClient.makeApiUrl("/api/v212/wow")
	suite.Equal(url, "https://testy-test.hu/api/v212/wow")
}

func (suite *ApiClientTestSuite) TestMakeApiUrl_noPrefix() {
	url := suite.apiClient.makeApiUrl("/wow")
	suite.Equal(url, "https://testy-test.hu/api/v1/wow")
}
