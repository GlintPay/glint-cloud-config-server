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
		{Name: "application-staging.yml", Source: EmptySource},
		{Name: "application-demo.yml", Source: EmptySource},
		{Name: "application-staging-us.yml", Source: EmptySource},
		{Name: "application-demo-us.yml", Source: EmptySource},

		{Name: "adjust-events.yml", Source: EmptySource},
		{Name: "adjust-events-junk.yml", Source: EmptySource},
		{Name: "adjust-events-staging.yml", Source: EmptySource},
		{Name: "adjust-events-demo.yml", Source: EmptySource},
		{Name: "adjust-events-staging-us.yml", Source: EmptySource},
		{Name: "adjust-events-demo-us.yml", Source: EmptySource},
	}

	// Ordering of inputs is not random in reality; this is just to clarify the effect of the sorting algo
	sources := []PropertySource{
		{Name: "adjust-events-demo-us.yml", Source: EmptySource},
		{Name: "application-junk.yml", Source: EmptySource},
		{Name: "adjust-events-staging-us.yml", Source: EmptySource},
		{Name: "application-demo-us.yml", Source: EmptySource},
		{Name: "adjust-events-staging.yml", Source: EmptySource},
		{Name: "application-demo.yml", Source: EmptySource},
		{Name: "adjust-events-demo.yml", Source: EmptySource},
		{Name: "adjust-events.yml", Source: EmptySource},
		{Name: "adjust-events-junk.yml", Source: EmptySource},
		{Name: "application-staging-us.yml", Source: EmptySource},
		{Name: "application-staging.yml", Source: EmptySource},
		{Name: "application.yml", Source: EmptySource},
	}

	sorter := Sorter{AppNames: []string{"adjust-events"}, Profiles: []string{"demo-us", "staging-us", "demo", "staging"}, Sources: sources}
	sort.SliceStable(sources, sorter.Sort())

	assert.Equal(t, expected, sources)
}
