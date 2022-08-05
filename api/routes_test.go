package api

import (
	"bytes"
	"fmt"
	"github.com/GlintPay/gccs/backend"
	"github.com/GlintPay/gccs/backend/git"
	"github.com/GlintPay/gccs/config"
	"github.com/GlintPay/gccs/logging"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	goGit "github.com/go-git/go-git/v5"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

var traceServerName = fmt.Sprintf("server-%d", rand.Int())

//goland:noinspection GoUnhandledErrorResult
func Test_routesHierarchical(t *testing.T) {
	gitDir, err := os.MkdirTemp("", "*")
	assert.NoError(t, err)
	defer os.Remove(gitDir)

	repo, err := goGit.PlainInit(gitDir, false)
	assert.NoError(t, err)

	wt, err := repo.Worktree()
	assert.NoError(t, err)

	setUpFiles(t, gitDir, wt)

	var backends backend.Backends
	backends = append(backends, &git.Backend{
		Repo: repo,
	})

	expectedVersion := _getHash(repo)

	router := setUpRouter(t, backends, false)

	//////////////////////////////////////////////////////

	tests := []ExampleRequest{
		{
			method:     "GET",
			url:        "/xxxxx", // don't refresh git
			statusCode: 404,
			jsonOutput: `404 page not found`,
		},
		{
			method:     "GET",
			url:        "/accounts/production?resolve=true&norefresh", // don't refresh git
			statusCode: 200,
			jsonOutput: `{"a":"b123","accountstuff":{"currencies":["DEF","GHI","JKL"],"val":"xxx"},"b":"c234","c":"d344","currencies":["USD","EUR","ABC"],"site":{"interval":5,"retries":5,"timeout":5,"url":"https://live.com"},"supportedCurrencies":{"ABC":{},"EUR":{},"GBP":{}}}`,
			headers: http.Header{
				"Content-Type":                          []string{"application/json"},
				"X-Resolution-Version":                  []string{expectedVersion},
				"X-Resolution-Label":                    []string{""},
				"X-Resolution-Name":                     []string{"accounts"},
				"X-Resolution-Profiles":                 []string{"production"},
				"X-Resolution-Precedencedisplaymessage": []string{"accounts-production.yaml > accounts.yaml > application-production.yaml > application.yaml"},
			},
		},
		{
			method:     "GET",
			url:        "/accounts/production?resolve=true&norefresh&pretty=true", // don't refresh git
			statusCode: 200,
			jsonOutput: `{
  "a": "b123",
  "accountstuff": {
    "currencies": [
      "DEF",
      "GHI",
      "JKL"
    ],
    "val": "xxx"
  },
  "b": "c234",
  "c": "d344",
  "currencies": [
    "USD",
    "EUR",
    "ABC"
  ],
  "site": {
    "interval": 5,
    "retries": 5,
    "timeout": 5,
    "url": "https://live.com"
  },
  "supportedCurrencies": {
    "ABC": {},
    "EUR": {},
    "GBP": {}
  }
}`,
			headers: http.Header{
				"Content-Type":                          []string{"application/json"},
				"X-Resolution-Version":                  []string{expectedVersion},
				"X-Resolution-Label":                    []string{""},
				"X-Resolution-Name":                     []string{"accounts"},
				"X-Resolution-Profiles":                 []string{"production"},
				"X-Resolution-Precedencedisplaymessage": []string{"accounts-production.yaml > accounts.yaml > application-production.yaml > application.yaml"},
			},
		},
		{
			method:     "GET",
			url:        "/accounts/production?resolve=true&norefresh&logResponses=true", // don't refresh git
			statusCode: 200,
			jsonOutput: `{"a":"b123","accountstuff":{"currencies":["DEF","GHI","JKL"],"val":"xxx"},"b":"c234","c":"d344","currencies":["USD","EUR","ABC"],"site":{"interval":5,"retries":5,"timeout":5,"url":"https://live.com"},"supportedCurrencies":{"ABC":{},"EUR":{},"GBP":{}}}`,
			headers: http.Header{
				"Content-Type":                          []string{"application/json"},
				"X-Resolution-Version":                  []string{expectedVersion},
				"X-Resolution-Label":                    []string{""},
				"X-Resolution-Name":                     []string{"accounts"},
				"X-Resolution-Profiles":                 []string{"production"},
				"X-Resolution-Precedencedisplaymessage": []string{"accounts-production.yaml > accounts.yaml > application-production.yaml > application.yaml"},
			},
		},
		{
			method:     "GET",
			url:        "/somethingelse/production?resolve=true&norefresh", // don't refresh git
			statusCode: 200,
			jsonOutput: `{"a":"b123","b":"c234","c":"d344"}`,
			headers: http.Header{
				"Content-Type":                          []string{"application/json"},
				"X-Resolution-Version":                  []string{expectedVersion},
				"X-Resolution-Label":                    []string{""},
				"X-Resolution-Name":                     []string{"somethingelse"},
				"X-Resolution-Profiles":                 []string{"production"},
				"X-Resolution-Precedencedisplaymessage": []string{"application-production.yaml > application.yaml"},
			},
		},
		{
			method:     "GET",
			url:        "/accounts/local?resolve=true&norefresh", // don't refresh git
			statusCode: 200,
			jsonOutput: `{"a":"b","accountstuff":{"currencies":["DEF","GHI","JKL"],"val":"xxx"},"b":"c","c":"d","currencies":["USD","EUR","ABC"],"site":{"retries":0,"timeout":50,"url":"https://test.com"},"supportedCurrencies":{"ABC":{},"EUR":{},"GBP":{}}}`,
			headers: http.Header{
				"Content-Type":                          []string{"application/json"},
				"X-Resolution-Version":                  []string{expectedVersion},
				"X-Resolution-Label":                    []string{""},
				"X-Resolution-Name":                     []string{"accounts"},
				"X-Resolution-Profiles":                 []string{"local"},
				"X-Resolution-Precedencedisplaymessage": []string{"accounts.yaml > application.yaml"},
			},
		},
		{
			method:     "GET",
			url:        "/somethingelse/local?resolve=true&norefresh", // don't refresh git
			statusCode: 200,
			jsonOutput: `{"a":"b","b":"c","c":"d"}`,
			headers: http.Header{
				"Content-Type":                          []string{"application/json"},
				"X-Resolution-Version":                  []string{expectedVersion},
				"X-Resolution-Label":                    []string{""},
				"X-Resolution-Name":                     []string{"somethingelse"},
				"X-Resolution-Profiles":                 []string{"local"},
				"X-Resolution-Precedencedisplaymessage": []string{"application.yaml"},
			},
		},
		{
			method:     "PATCH",
			url:        "/somethingelse/local?resolve=true&norefresh", // don't refresh git
			body:       strings.NewReader(`{"cloudconfig.serviceName":"xxx","cloudconfig.serviceNameUnderscores":"yyy"}`),
			statusCode: 200,
			jsonOutput: `{"a":"b","b":"c","c":"d","cloudconfig.serviceName":"xxx","cloudconfig.serviceNameUnderscores":"yyy"}`,
		},
		{
			method:     "PATCH",
			url:        "/somethingelse/local?resolve=true&norefresh",                                // don't refresh git
			body:       strings.NewReader(`{"^a":"original-val","^c":"original-val-2","b":"new-b"}`), // first two will be overwritten
			statusCode: 200,
			jsonOutput: `{"a":"b","b":"new-b","c":"d"}`,
		},
		{
			method:     "PATCH",
			url:        "/somethingelse/local?resolve=true&norefresh", // don't refresh git
			body:       strings.NewReader(``),
			statusCode: 200,
			jsonOutput: `{"a":"b","b":"c","c":"d"}`,
		},
		{
			method:     "PATCH",
			url:        "/somethingelse/local?resolve=true&norefresh", // don't refresh git
			body:       strings.NewReader(`junk`),
			statusCode: 500,
			jsonOutput: `{"message":"Unparseable JSON: invalid character 'j' looking for beginning of value"}`,
		},
		{
			method: "PATCH",
			url:    "/somethingelse/local?resolve=true&norefresh", // don't refresh git
			// no body
			statusCode: 200,
			jsonOutput: `{"a":"b","b":"c","c":"d"}`,
		},
		{
			method:     "GET",
			url:        "/accounts/local?norefresh", // don't refresh git
			statusCode: 200,
			jsonOutput: `{"name":"accounts","profiles":["local"],"label":"","version":"` + expectedVersion + `","state":"","propertySources":[{"name":"/accounts.yaml","source":{"accountstuff":{"currencies":["DEF","GHI","JKL"],"val":"xxx"},"currencies":["USD","EUR","ABC"],"site":{"retries":0,"timeout":50,"url":"https://test.com"},"supportedCurrencies":{"ABC":{},"EUR":{},"GBP":{}}}},{"name":"/application.yaml","source":{"a":"b","b":"c","c":"d"}}]}`,
			headers:    http.Header{"Content-Type": []string{"application/json"}},
		},
		{
			method:     "GET",
			url:        "/accounts/other,local?norefresh", // don't refresh git
			statusCode: 200,
			jsonOutput: `{"name":"accounts","profiles":["other","local"],"label":"","version":"` + expectedVersion + `","state":"","propertySources":[{"name":"/accounts.yaml","source":{"accountstuff":{"currencies":["DEF","GHI","JKL"],"val":"xxx"},"currencies":["USD","EUR","ABC"],"site":{"retries":0,"timeout":50,"url":"https://test.com"},"supportedCurrencies":{"ABC":{},"EUR":{},"GBP":{}}}},{"name":"/application-other.yaml","source":{"a":"b1","c":"d2"}},{"name":"/application.yaml","source":{"a":"b","b":"c","c":"d"}}]}`,
			headers:    http.Header{"Content-Type": []string{"application/json"}},
		},
		{
			method:     "GET",
			url:        "/accounts/other,local?resolve=true&norefresh", // don't refresh git
			statusCode: 200,
			jsonOutput: `{"a":"b1","accountstuff":{"currencies":["DEF","GHI","JKL"],"val":"xxx"},"b":"c","c":"d2","currencies":["USD","EUR","ABC"],"site":{"retries":0,"timeout":50,"url":"https://test.com"},"supportedCurrencies":{"ABC":{},"EUR":{},"GBP":{}}}`,
			headers: http.Header{
				"Content-Type":                          []string{"application/json"},
				"X-Resolution-Version":                  []string{expectedVersion},
				"X-Resolution-Label":                    []string{""},
				"X-Resolution-Name":                     []string{"accounts"},
				"X-Resolution-Profiles":                 []string{"other,local"},
				"X-Resolution-Precedencedisplaymessage": []string{"accounts.yaml > application-other.yaml > application.yaml"},
			},
		},
		{
			method:     "GET",
			url:        "/accounts,security/other,local?resolve=true&norefresh", // don't refresh git
			statusCode: 200,
			jsonOutput: `{"a":"b1","accountstuff":{"currencies":["DEF","GHI","JKL"],"val":"xxx"},"b":"c","c":"d2","currencies":["USD","EUR","ABC"],"password":"test123","site":{"retries":0,"timeout":50,"url":"https://test.com"},"supportedCurrencies":{"ABC":{},"EUR":{},"GBP":{}}}`,
			headers: http.Header{
				"Content-Type":                          []string{"application/json"},
				"X-Resolution-Version":                  []string{expectedVersion},
				"X-Resolution-Label":                    []string{""},
				"X-Resolution-Name":                     []string{"accounts,security"},
				"X-Resolution-Profiles":                 []string{"other,local"},
				"X-Resolution-Precedencedisplaymessage": []string{"accounts.yaml > security.yaml > application-other.yaml > application.yaml"},
			},
		},
		{
			method:     "GET",
			url:        "/accounts/junk?norefresh", // don't refresh git
			statusCode: 500,
			jsonOutput: `{"message":"error unmarshaling JSON: while decoding JSON: json: cannot unmarshal string into Go value of type map[string]interface {}"}`,
			headers:    http.Header{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			validateRequest(t, tt, tt.jsonOutput, router, "")
		})
	}
}

func Test_routesFlattened(t *testing.T) {
	gitDir, err := os.MkdirTemp("", "*")
	assert.NoError(t, err)
	//goland:noinspection GoUnhandledErrorResult
	defer os.Remove(gitDir)

	repo, err := goGit.PlainInit(gitDir, false)
	assert.NoError(t, err)

	wt, err := repo.Worktree()
	assert.NoError(t, err)

	setUpFiles(t, gitDir, wt)

	var backends backend.Backends
	backends = append(backends, &git.Backend{
		Repo: repo,
	})

	expectedVersion := _getHash(repo)

	router := setUpRouter(nil, backends, false)

	//////////////////////////////////////////////////////

	tests := []ExampleRequest{
		{
			method:     "GET",
			url:        "/xxxxx", // don't refresh git
			statusCode: 404,
			jsonOutput: `404 page not found`,
		},
		{
			method:        "GET",
			url:           "/accounts/production?resolve=true&norefresh", // don't refresh git
			statusCode:    200,
			jsonOutput:    `{"a":"b123","accountstuff.currencies":["DEF","GHI","JKL"],"accountstuff.val":"xxx","b":"c234","c":"d344","currencies":["USD","EUR","ABC"],"site.interval":5,"site.retries":5,"site.timeout":5,"site.url":"https://live.com","supportedCurrencies.ABC":{},"supportedCurrencies.EUR":{},"supportedCurrencies.GBP":{}}`,
			jsonOutputAlt: `{"a":"b123","accountstuff.currencies[0]":"DEF","accountstuff.currencies[1]":"GHI","accountstuff.currencies[2]":"JKL","accountstuff.val":"xxx","b":"c234","c":"d344","currencies[0]":"USD","currencies[1]":"EUR","currencies[2]":"ABC","site.interval":5,"site.retries":5,"site.timeout":5,"site.url":"https://live.com","supportedCurrencies.ABC":{},"supportedCurrencies.EUR":{},"supportedCurrencies.GBP":{}}`,
			headers: http.Header{
				"Content-Type":                          []string{"application/json"},
				"X-Resolution-Version":                  []string{expectedVersion},
				"X-Resolution-Label":                    []string{""},
				"X-Resolution-Name":                     []string{"accounts"},
				"X-Resolution-Profiles":                 []string{"production"},
				"X-Resolution-Precedencedisplaymessage": []string{"accounts-production.yaml > accounts.yaml > application-production.yaml > application.yaml"},
			},
		},
		{
			method:        "GET",
			url:           "/somethingelse/production?resolve=true&norefresh", // don't refresh git
			statusCode:    200,
			jsonOutput:    `{"a":"b123","b":"c234","c":"d344"}`,
			jsonOutputAlt: `{"a":"b123","b":"c234","c":"d344"}`,
			headers: http.Header{
				"Content-Type":                          []string{"application/json"},
				"X-Resolution-Version":                  []string{expectedVersion},
				"X-Resolution-Label":                    []string{""},
				"X-Resolution-Name":                     []string{"somethingelse"},
				"X-Resolution-Profiles":                 []string{"production"},
				"X-Resolution-Precedencedisplaymessage": []string{"application-production.yaml > application.yaml"},
			},
		},
		{
			method:        "GET",
			url:           "/accounts/local?resolve=true&norefresh", // don't refresh git
			statusCode:    200,
			jsonOutput:    `{"a":"b","accountstuff.currencies":["DEF","GHI","JKL"],"accountstuff.val":"xxx","b":"c","c":"d","currencies":["USD","EUR","ABC"],"site.retries":0,"site.timeout":50,"site.url":"https://test.com","supportedCurrencies.ABC":{},"supportedCurrencies.EUR":{},"supportedCurrencies.GBP":{}}`,
			jsonOutputAlt: `{"a":"b","accountstuff.currencies[0]":"DEF","accountstuff.currencies[1]":"GHI","accountstuff.currencies[2]":"JKL","accountstuff.val":"xxx","b":"c","c":"d","currencies[0]":"USD","currencies[1]":"EUR","currencies[2]":"ABC","site.retries":0,"site.timeout":50,"site.url":"https://test.com","supportedCurrencies.ABC":{},"supportedCurrencies.EUR":{},"supportedCurrencies.GBP":{}}`,
			headers: http.Header{
				"Content-Type":                          []string{"application/json"},
				"X-Resolution-Version":                  []string{expectedVersion},
				"X-Resolution-Label":                    []string{""},
				"X-Resolution-Name":                     []string{"accounts"},
				"X-Resolution-Profiles":                 []string{"local"},
				"X-Resolution-Precedencedisplaymessage": []string{"accounts.yaml > application.yaml"},
			},
		},
		{
			method:     "GET",
			url:        "/somethingelse/local?resolve=true&norefresh", // don't refresh git
			statusCode: 200,
			jsonOutput: `{"a":"b","b":"c","c":"d"}`,
			headers: http.Header{
				"Content-Type":                          []string{"application/json"},
				"X-Resolution-Version":                  []string{expectedVersion},
				"X-Resolution-Label":                    []string{""},
				"X-Resolution-Name":                     []string{"somethingelse"},
				"X-Resolution-Profiles":                 []string{"local"},
				"X-Resolution-Precedencedisplaymessage": []string{"application.yaml"},
			},
		},
		{
			method:     "PATCH",
			url:        "/somethingelse/local?resolve=true&norefresh", // don't refresh git
			body:       strings.NewReader(`{"cloudconfig.serviceName":"xxx","cloudconfig.serviceNameUnderscores":"yyy"}`),
			statusCode: 200,
			jsonOutput: `{"a":"b","b":"c","c":"d","cloudconfig.serviceName":"xxx","cloudconfig.serviceNameUnderscores":"yyy"}`,
		},
		{
			method:     "PATCH",
			url:        "/somethingelse/local?resolve=true&norefresh",                            // don't refresh git
			body:       strings.NewReader(`{"^a":"original-val","^c":"original-val-2","e":"f"}`), // first two will be overwritten
			statusCode: 200,
			jsonOutput: `{"a":"b","b":"c","c":"d","e":"f"}`,
		},
		{
			method:        "GET",
			url:           "/accounts/local?norefresh", // don't refresh git
			statusCode:    200,
			jsonOutput:    `{"name":"accounts","profiles":["local"],"label":"","version":"` + expectedVersion + `","state":"","propertySources":[{"name":"/accounts.yaml","source":{"accountstuff.currencies":["DEF","GHI","JKL"],"accountstuff.val":"xxx","currencies":["USD","EUR","ABC"],"site.retries":0,"site.timeout":50,"site.url":"https://test.com","supportedCurrencies.ABC":{},"supportedCurrencies.EUR":{},"supportedCurrencies.GBP":{}}},{"name":"/application.yaml","source":{"a":"b","b":"c","c":"d"}}]}`,
			jsonOutputAlt: `{"name":"accounts","profiles":["local"],"label":"","version":"` + expectedVersion + `","state":"","propertySources":[{"name":"/accounts.yaml","source":{"accountstuff.currencies[0]":"DEF","accountstuff.currencies[1]":"GHI","accountstuff.currencies[2]":"JKL","accountstuff.val":"xxx","currencies[0]":"USD","currencies[1]":"EUR","currencies[2]":"ABC","site.retries":0,"site.timeout":50,"site.url":"https://test.com","supportedCurrencies.ABC":{},"supportedCurrencies.EUR":{},"supportedCurrencies.GBP":{}}},{"name":"/application.yaml","source":{"a":"b","b":"c","c":"d"}}]}`,
			headers:       http.Header{"Content-Type": []string{"application/json"}},
		},
		{
			method:        "GET",
			url:           "/accounts/other,local?resolve=false&norefresh", // don't refresh git
			statusCode:    200,
			jsonOutput:    `{"name":"accounts","profiles":["other","local"],"label":"","version":"` + expectedVersion + `","state":"","propertySources":[{"name":"/accounts.yaml","source":{"accountstuff.currencies":["DEF","GHI","JKL"],"accountstuff.val":"xxx","currencies":["USD","EUR","ABC"],"site.retries":0,"site.timeout":50,"site.url":"https://test.com","supportedCurrencies.ABC":{},"supportedCurrencies.EUR":{},"supportedCurrencies.GBP":{}}},{"name":"/application-other.yaml","source":{"a":"b1","c":"d2"}},{"name":"/application.yaml","source":{"a":"b","b":"c","c":"d"}}]}`,
			jsonOutputAlt: `{"name":"accounts","profiles":["other","local"],"label":"","version":"` + expectedVersion + `","state":"","propertySources":[{"name":"/accounts.yaml","source":{"accountstuff.currencies[0]":"DEF","accountstuff.currencies[1]":"GHI","accountstuff.currencies[2]":"JKL","accountstuff.val":"xxx","currencies[0]":"USD","currencies[1]":"EUR","currencies[2]":"ABC","site.retries":0,"site.timeout":50,"site.url":"https://test.com","supportedCurrencies.ABC":{},"supportedCurrencies.EUR":{},"supportedCurrencies.GBP":{}}},{"name":"/application-other.yaml","source":{"a":"b1","c":"d2"}},{"name":"/application.yaml","source":{"a":"b","b":"c","c":"d"}}]}`,
			headers:       http.Header{"Content-Type": []string{"application/json"}},
		},
		{
			method:        "GET",
			url:           "/accounts/other,local?resolve=true&norefresh", // don't refresh git
			statusCode:    200,
			jsonOutput:    `{"a":"b1","accountstuff.currencies":["DEF","GHI","JKL"],"accountstuff.val":"xxx","b":"c","c":"d2","currencies":["USD","EUR","ABC"],"site.retries":0,"site.timeout":50,"site.url":"https://test.com","supportedCurrencies.ABC":{},"supportedCurrencies.EUR":{},"supportedCurrencies.GBP":{}}`,
			jsonOutputAlt: `{"a":"b1","accountstuff.currencies[0]":"DEF","accountstuff.currencies[1]":"GHI","accountstuff.currencies[2]":"JKL","accountstuff.val":"xxx","b":"c","c":"d2","currencies[0]":"USD","currencies[1]":"EUR","currencies[2]":"ABC","site.retries":0,"site.timeout":50,"site.url":"https://test.com","supportedCurrencies.ABC":{},"supportedCurrencies.EUR":{},"supportedCurrencies.GBP":{}}`,
			headers: http.Header{
				"Content-Type":                          []string{"application/json"},
				"X-Resolution-Version":                  []string{expectedVersion},
				"X-Resolution-Label":                    []string{""},
				"X-Resolution-Name":                     []string{"accounts"},
				"X-Resolution-Profiles":                 []string{"other,local"},
				"X-Resolution-Precedencedisplaymessage": []string{"accounts.yaml > application-other.yaml > application.yaml"},
			},
		},
		{
			method:        "GET",
			url:           "/accounts,security/other,local?resolve=true&norefresh", // don't refresh git
			statusCode:    200,
			jsonOutput:    `{"a":"b1","accountstuff.currencies":["DEF","GHI","JKL"],"accountstuff.val":"xxx","b":"c","c":"d2","currencies":["USD","EUR","ABC"],"password":"test123","site.retries":0,"site.timeout":50,"site.url":"https://test.com","supportedCurrencies.ABC":{},"supportedCurrencies.EUR":{},"supportedCurrencies.GBP":{}}`,
			jsonOutputAlt: `{"a":"b1","accountstuff.currencies[0]":"DEF","accountstuff.currencies[1]":"GHI","accountstuff.currencies[2]":"JKL","accountstuff.val":"xxx","b":"c","c":"d2","currencies[0]":"USD","currencies[1]":"EUR","currencies[2]":"ABC","password":"test123","site.retries":0,"site.timeout":50,"site.url":"https://test.com","supportedCurrencies.ABC":{},"supportedCurrencies.EUR":{},"supportedCurrencies.GBP":{}}`,
			headers: http.Header{
				"Content-Type":                          []string{"application/json"},
				"X-Resolution-Version":                  []string{expectedVersion},
				"X-Resolution-Label":                    []string{""},
				"X-Resolution-Name":                     []string{"accounts,security"},
				"X-Resolution-Profiles":                 []string{"other,local"},
				"X-Resolution-Precedencedisplaymessage": []string{"accounts.yaml > security.yaml > application-other.yaml > application.yaml"},
			},
		},
		{
			method:     "GET",
			url:        "/accounts/junk?norefresh", // don't refresh git
			statusCode: 500,
			jsonOutput: `{"message":"error unmarshaling JSON: while decoding JSON: json: cannot unmarshal string into Go value of type map[string]interface {}"}`,
			headers:    http.Header{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			validateRequest(t, tt, tt.jsonOutput, router, "&flatten=true&flattenLists=false")

			if tt.jsonOutputAlt != "" {
				validateRequest(t, tt, tt.jsonOutputAlt, router, "&flatten=true&flattenLists=true")
			}
		})
	}
}

func Test_routesFlattenedNested(t *testing.T) {
	gitDir, err := os.MkdirTemp("", "*")
	assert.NoError(t, err)
	//goland:noinspection GoUnhandledErrorResult
	defer os.Remove(gitDir)

	repo, err := goGit.PlainInit(gitDir, false)
	assert.NoError(t, err)

	wt, err := repo.Worktree()
	assert.NoError(t, err)

	_writeGitFile(t, gitDir, wt, "accounts.yaml", `
site:
  url: https://test.com
  timeout: 50
  retries: 0
currencies:
  - USD
  - EUR
  - ABC
accountstuff:
  val: xxx
  currencies:
    - name: DEF
      country: djibouti
      x: yy
    - name: GHI
      country: ghana
      x: z
    - name: JKL
      country: jersey
      x: a
`)

	var backends backend.Backends
	backends = append(backends, &git.Backend{
		Repo: repo,
	})

	expectedVersion := _getHash(repo)

	router := setUpRouter(nil, backends, false)

	//////////////////////////////////////////////////////

	tests := []ExampleRequest{
		{
			method:        "GET",
			url:           "/accounts/local?norefresh", // don't refresh git
			statusCode:    200,
			jsonOutput:    `{"name":"accounts","profiles":["local"],"label":"","version":"` + expectedVersion + `","state":"","propertySources":[{"name":"/accounts.yaml","source":{"accountstuff.currencies":[{"country":"djibouti","name":"DEF","x":"yy"},{"country":"ghana","name":"GHI","x":"z"},{"country":"jersey","name":"JKL","x":"a"}],"accountstuff.val":"xxx","currencies":["USD","EUR","ABC"],"site.retries":0,"site.timeout":50,"site.url":"https://test.com"}}]}`,
			jsonOutputAlt: `{"name":"accounts","profiles":["local"],"label":"","version":"` + expectedVersion + `","state":"","propertySources":[{"name":"/accounts.yaml","source":{"accountstuff.currencies[0].country":"djibouti","accountstuff.currencies[0].name":"DEF","accountstuff.currencies[0].x":"yy","accountstuff.currencies[1].country":"ghana","accountstuff.currencies[1].name":"GHI","accountstuff.currencies[1].x":"z","accountstuff.currencies[2].country":"jersey","accountstuff.currencies[2].name":"JKL","accountstuff.currencies[2].x":"a","accountstuff.val":"xxx","currencies[0]":"USD","currencies[1]":"EUR","currencies[2]":"ABC","site.retries":0,"site.timeout":50,"site.url":"https://test.com"}}]}`,
			headers:       http.Header{"Content-Type": []string{"application/json"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			validateRequest(t, tt, tt.jsonOutput, router, "&flatten=true&flattenLists=false")

			if tt.jsonOutputAlt != "" {
				validateRequest(t, tt, tt.jsonOutputAlt, router, "&flatten=true&flattenLists=true")
			}
		})
	}
}

//goland:noinspection GoUnhandledErrorResult
func Test_routesTraceEnabled(t *testing.T) {

	gitDir, err := os.MkdirTemp("", "*")
	assert.NoError(t, err)
	defer os.Remove(gitDir)

	repo, err := goGit.PlainInit(gitDir, false)
	assert.NoError(t, err)

	wt, err := repo.Worktree()
	assert.NoError(t, err)

	setUpFiles(t, gitDir, wt)

	var backends backend.Backends
	backends = append(backends, &git.Backend{
		Repo: repo,
	})

	//////////////////////////////////////////////////////

	sr := tracetest.NewSpanRecorder()
	tracerProvider := sdktrace.NewTracerProvider()
	tracerProvider.RegisterSpanProcessor(sr)
	otel.SetTracerProvider(tracerProvider)

	router := setUpRouter(t, backends, true)

	//////////////////////////////////////////////////////

	tests := []ExampleRequest{
		{
			method:     "GET",
			url:        "/accounts/production?resolve=true&norefresh", // don't refresh git
			statusCode: 200,
			jsonOutput: `{"a":"b123","accountstuff":{"currencies":["DEF","GHI","JKL"],"val":"xxx"},"b":"c234","c":"d344","currencies":["USD","EUR","ABC"],"site":{"interval":5,"retries":5,"timeout":5,"url":"https://live.com"},"supportedCurrencies":{"ABC":{},"EUR":{},"GBP":{}}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			validateRequest(t, tt, tt.jsonOutput, router, "")

			require.Len(t, sr.Ended(), 3)

			assertSpan(t, sr.Ended()[0],
				"loadConfiguration",
				trace.SpanKindServer,
				[]attribute.KeyValue{}...,
			)

			assertSpan(t, sr.Ended()[1],
				"reconcile",
				trace.SpanKindServer,
				[]attribute.KeyValue{}...,
			)

			assertSpan(t, sr.Ended()[2],
				"/{application}/{profiles}",
				trace.SpanKindServer,
				attribute.String("http.server_name", traceServerName),
				attribute.Int("http.status_code", http.StatusOK),
				attribute.String("http.method", "GET"),
				attribute.String("http.target", "/accounts/production?resolve=true&norefresh"),
				attribute.String("http.route", "/{application}/{profiles}"),
			)
		})
	}
}

//goland:noinspection GoUnhandledErrorResult
func Test_routesResponseLoggingEnabled(t *testing.T) {

	gitDir, err := os.MkdirTemp("", "*")
	assert.NoError(t, err)
	defer os.Remove(gitDir)

	repo, err := goGit.PlainInit(gitDir, false)
	assert.NoError(t, err)

	wt, err := repo.Worktree()
	assert.NoError(t, err)

	setUpFiles(t, gitDir, wt)

	var backends backend.Backends
	backends = append(backends, &git.Backend{
		Repo: repo,
	})

	router := setUpRouter(t, backends, true)

	//////////////////////////////////////////////////////

	tests := []ExampleRequest{
		{
			method:     "GET",
			url:        "/accounts/production?resolve=true&norefresh&logResponses=true", // don't refresh git
			statusCode: 200,
			jsonOutput: `{"a":"b123","accountstuff":{"currencies":["DEF","GHI","JKL"],"val":"xxx"},"b":"c234","c":"d344","currencies":["USD","EUR","ABC"],"site":{"interval":5,"retries":5,"timeout":5,"url":"https://live.com"},"supportedCurrencies":{"ABC":{},"EUR":{},"GBP":{}}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			var str bytes.Buffer
			log.Logger = zerolog.New(&str).With().Timestamp().Logger()

			validateRequest(t, tt, tt.jsonOutput, router, "")

			logOutput := str.String()
			assert.Contains(t, logOutput, "Requesting: [accounts]/[production]/[{}]")
			assert.Contains(t, logOutput, "Response: {")
		})
	}
}

//goland:noinspection GoUnhandledErrorResult
func Test_routesResponseErrorsLogged(t *testing.T) {

	gitDir, err := os.MkdirTemp("", "*")
	assert.NoError(t, err)
	defer os.Remove(gitDir)

	repo, err := goGit.PlainInit(gitDir, false)
	assert.NoError(t, err)

	wt, err := repo.Worktree()
	assert.NoError(t, err)

	_writeGitFile(t, gitDir, wt, "application-junk.yaml", `junk sdasdasda`)

	setUpFiles(t, gitDir, wt)

	var backends backend.Backends
	backends = append(backends, &git.Backend{
		Repo: repo,
	})

	router := setUpRouter(t, backends, true)

	//////////////////////////////////////////////////////

	tests := []ExampleRequest{
		{
			method:     "GET",
			url:        "/accounts/junk?resolve=true&norefresh", // don't refresh git
			statusCode: 500,
			jsonOutput: `{"message":"error unmarshaling JSON: while decoding JSON: json: cannot unmarshal string into Go value of type map[string]interface {}"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			var str bytes.Buffer
			logging.Setup(&str)

			validateRequest(t, tt, tt.jsonOutput, router, "")

			logOutput := str.String()
			assert.Contains(t, logOutput, "error unmarshaling JSON: while decoding JSON")
			assert.Contains(t, logOutput, "api/routes.go") // caller
		})
	}
}

func validateRequest(t *testing.T, tt ExampleRequest, jsonOutput string, router http.Handler, urlSuffix string) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(tt.method, tt.url+urlSuffix, tt.body) // don't refresh git

	router.ServeHTTP(rr, req)

	assert.Equal(t, tt.statusCode, rr.Code)
	assert.Equal(t, jsonOutput, strings.TrimSpace(rr.Body.String()))

	if tt.headers != nil {
		assert.Equal(t, tt.headers, rr.Header())
	}
}

func setUpRouter(t *testing.T, bs backend.Backends, traceEnabled bool) *chi.Mux {
	router := chi.NewRouter()
	router.Use(middleware.StripSlashes)

	routing := Routing{
		ServerName:   traceServerName,
		ParentRouter: router,

		Backends: bs,
		AppConfig: config.ApplicationConfiguration{
			Defaults: config.Defaults{
				FlattenHierarchicalConfig: false,
				ResolvePropertySources:    false,
			},
			Tracing: config.Tracing{
				Enabled: traceEnabled,
			},
		},
	}

	router.Route("/", func(r chi.Router) {
		err := routing.SetupFunctionalRoutes(r)
		assert.NoError(t, err)
	})
	return router
}

func setUpFiles(t *testing.T, gitDir string, wt *goGit.Worktree) {
	_writeGitFile(t, gitDir, wt, "accounts.yaml", `
site:
  url: https://test.com
  timeout: 50
  retries: 0
currencies:
  - USD
  - EUR
  - ABC

supportedCurrencies:
  GBP: {}
  EUR: {}
  ABC: {}

accountstuff:
  val: xxx
  currencies:
    - DEF
    - GHI
    - JKL
`)

	_writeGitFile(t, gitDir, wt, "accounts-production.yaml", `
site:
  url: https://live.com
  timeout: 5
  retries: 5
  interval: 5
`)

	_writeGitFile(t, gitDir, wt, "application-production.yaml", `
a: b123
b: c234
c: d344
`)

	_writeGitFile(t, gitDir, wt, "application-junk.yaml", `junk sdasdasda`)

	_writeGitFile(t, gitDir, wt, "application-other.yaml", `
a: b1
c: d2
`)

	_writeGitFile(t, gitDir, wt, "security.yaml", `
password: test123
`)

	_writeGitFile(t, gitDir, wt, "application.yaml", `
a: b
b: c
c: d
`)

}

type ExampleRequest struct {
	method        string
	url           string
	body          io.Reader
	statusCode    int
	jsonOutput    string
	jsonOutputAlt string
	headers       http.Header
}

func assertSpan(t *testing.T, span sdktrace.ReadOnlySpan, name string, kind trace.SpanKind, attrs ...attribute.KeyValue) {
	assert.Equal(t, name, span.Name())
	assert.Equal(t, kind, span.SpanKind())

	got := make(map[attribute.Key]attribute.Value, len(span.Attributes()))
	for _, a := range span.Attributes() {
		got[a.Key] = a.Value
	}
	for _, want := range attrs {
		if !assert.Contains(t, got, want.Key) {
			continue
		}
		assert.Equal(t, got[want.Key], want.Value)
	}
}
