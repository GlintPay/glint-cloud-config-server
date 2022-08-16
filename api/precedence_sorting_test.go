package api

import (
	"github.com/stretchr/testify/assert"
	"sort"
	"testing"
)

var EmptySource = map[string]interface{}{}

func TestBasic(t *testing.T) {
	expected := []PropertySource{
		{Name: "application.yml", Source: EmptySource},
		{Name: "application-junk.yml", Source: EmptySource},
		{Name: "application-test.yml", Source: EmptySource},
		{Name: "application-demo.yml", Source: EmptySource},
		{Name: "application-test-us.yml", Source: EmptySource},
		{Name: "application-demo-us.yml", Source: EmptySource},

		{Name: "other-service.yml", Source: EmptySource},
		{Name: "other-service-junk.yml", Source: EmptySource},
		{Name: "other-service-test.yml", Source: EmptySource},
		{Name: "other-service-demo.yml", Source: EmptySource},
		{Name: "other-service-test-us.yml", Source: EmptySource},
		{Name: "other-service-demo-us.yml", Source: EmptySource},
	}

	// Ordering of inputs is not random in reality; this is just to clarify the effect of the sorting algo
	sources := []PropertySource{
		{Name: "other-service-demo-us.yml", Source: EmptySource},
		{Name: "application-junk.yml", Source: EmptySource},
		{Name: "other-service-test-us.yml", Source: EmptySource},
		{Name: "application-demo-us.yml", Source: EmptySource},
		{Name: "other-service-test.yml", Source: EmptySource},
		{Name: "application-demo.yml", Source: EmptySource},
		{Name: "other-service-demo.yml", Source: EmptySource},
		{Name: "other-service.yml", Source: EmptySource},
		{Name: "other-service-junk.yml", Source: EmptySource},
		{Name: "application-test-us.yml", Source: EmptySource},
		{Name: "application-test.yml", Source: EmptySource},
		{Name: "application.yml", Source: EmptySource},
	}

	sorter := Sorter{AppNames: []string{"other-service"}, Profiles: []string{"demo-us", "test-us", "demo", "test"}, Sources: sources}
	sort.SliceStable(sources, sorter.Sort())

	assert.Equal(t, expected, sources)
}
