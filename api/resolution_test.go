package api

import (
	"context"
	"github.com/GlintPay/gccs/internal/test"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_reconcileProperties(t *testing.T) {
	ctxt := context.Background()

	namePrefix := "git@github.com:GlintPay/cloud-config.git/"

	source := Source{Name: "test-app",
		PropertySources: []PropertySource{
			// deliberately misordered
			{Name: namePrefix + "backend.yml", Source: map[string]interface{}{"override": "3", "type": "backend"}},
			{Name: namePrefix + "application.yml", Source: map[string]interface{}{"override": "1", "glint.a": "b", "glint.b": "c", "glint.c": "d", "glint.name": "Default", "myService.host": "default", "myService.url": "https://${myService.host:UNUSED}.glintpay.com", "x.y.z": 123}},
			{Name: namePrefix + "myapp-mine.yml", Source: map[string]interface{}{"override": "7"}},
			{Name: namePrefix + "backend-mine.yml", Source: map[string]interface{}{"override": "5", "owner": "Mine"}},
			{Name: namePrefix + "myapp.yml", Source: map[string]interface{}{"override": "4"}},
			{Name: namePrefix + "backend-production.yml", Source: map[string]interface{}{"override": "6"}},
			{Name: namePrefix + "myapp-production.yml", Source: map[string]interface{}{"override": "8", "owner": "everyone"}},
			{Name: namePrefix + "application-production.yml", Source: map[string]interface{}{"override": "2", "glint.name": "Production", "myService.host": "production"}},
		}}

	resolver := Resolver{}
	resolved, md := resolver.ReconcileProperties(ctxt, []string{"myapp", "backend"}, []string{"production", "mine"}, InjectedProperties{}, &source)

	assert.Equal(t, "myapp-production.yml > myapp-mine.yml > myapp.yml > backend-production.yml > backend-mine.yml > backend.yml > application-production.yml > application.yml", md.PrecedenceDisplayMessage)

	assert.Equal(t,
		ResolvedConfigValues{"glint.a": "b", "glint.b": "c", "glint.c": "d", "glint.name": "Production", "myService.host": "production", "myService.url": "https://production.glintpay.com", "override": "8", "owner": "everyone", "type": "backend", "x.y.z": 123},
		resolved)
}

func Test_reconcileProperties_ListsReplacedNotMerged(t *testing.T) {
	ctxt := context.Background()

	tests := []SourcesRequest{
		{
			name: "three-level",
			sources: []PropertySource{
				{Name: "/application.yml", Source: map[string]interface{}{"list": []string{"a", "b", "c"}}},
				{Name: "/myapp-mine.yml", Source: map[string]interface{}{"list": []string{"y"}}},
				{Name: "/myapp.yml", Source: map[string]interface{}{"list": []string{"d", "x"}}},
			},
			expectation: ResolvedConfigValues{"list": []string{"y"}},
		},
		{
			name: "longer",
			sources: []PropertySource{
				{Name: "/application.yml", Source: map[string]interface{}{"list": []string{"a", "b", "c"}}},
				{Name: "/myapp-mine.yml", Source: map[string]interface{}{"list": []string{"y", "1", "2", "3", "4"}}},
			},
			expectation: ResolvedConfigValues{"list": []string{"y", "1", "2", "3", "4"}},
		},
		{
			name: "longer-2",
			sources: []PropertySource{
				{Name: "/application.yml", Source: map[string]interface{}{"list": []string{}}},
				{Name: "/myapp-mine.yml", Source: map[string]interface{}{"list": []string{"y", "1", "2", "3", "4"}}},
			},
			expectation: ResolvedConfigValues{"list": []string{"y", "1", "2", "3", "4"}},
		},
		{
			name: "shorter",
			sources: []PropertySource{
				{Name: "/application.yml", Source: map[string]interface{}{"list": []string{"a", "b", "c"}}},
				{Name: "/myapp-mine.yml", Source: map[string]interface{}{"list": []string{}}},
			},
			expectation: ResolvedConfigValues{"list": []string{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := Resolver{}
			resolved, _ := resolver.ReconcileProperties(ctxt, []string{"myapp"}, []string{"production", "mine"}, InjectedProperties{}, &Source{Name: "test-app", PropertySources: tt.sources})

			assert.Equal(t, tt.expectation, resolved)
		})
	}
}

type SourcesRequest struct {
	name        string
	sources     []PropertySource
	expectation ResolvedConfigValues
}

func Test_reconcilePropertiesWithInjection(t *testing.T) {
	ctxt := context.Background()

	source := Source{Name: "test-app",
		PropertySources: []PropertySource{
			{Name: "backend-mine.yml", Source: map[string]interface{}{"owner": "Mine"}},
			{Name: "backend.yml", Source: map[string]interface{}{"owner": "Unknown", "type": "backend"}},
			{Name: "application-development-eu.yml", Source: map[string]interface{}{"glint.name": "Production", "myService.host": "production"}},
			{Name: "application.yml", Source: map[string]interface{}{"glint.a": "b", "glint.b": "c", "glint.c": "d", "glint.name": "Default", "myService.host": "default", "myService.url": "https://${myService.host:UNUSED}.glintpay.com", "x.y.z": 123}},
		}}

	injections := InjectedProperties{ /* overwritten */ "^owner": "Mine" /* overwritten */, "^glint.name": "blah" /* good */, "^injectedServicename": "blah", "glint.c": "overwrite!"}

	resolver := Resolver{}
	resolved, md := resolver.ReconcileProperties(ctxt, []string{"test-app"}, []string{"production", "mine"}, injections, &source)

	assert.Equal(t, "backend-mine.yml > backend.yml > application-development-eu.yml > application.yml", md.PrecedenceDisplayMessage)

	assert.Equal(t,
		ResolvedConfigValues{"glint.a": "b", "glint.b": "c", "glint.c": "overwrite!", "glint.name": "Production", "myService.host": "production", "myService.url": "https://production.glintpay.com", "owner": "Mine", "injectedServicename": "blah", "type": "backend", "x.y.z": 123},
		resolved)
}

func Test_reconcileWithPointlessOverride(t *testing.T) {
	ctxt := context.Background()

	source := Source{Name: "test-app",
		PropertySources: []PropertySource{
			{Name: "backend-mine.yml", Source: map[string]interface{}{"owner": "Mine"}},
			{Name: "backend.yml", Source: map[string]interface{}{"owner": "Mine", "type": "backend"}},
		}}

	resolver := Resolver{}
	resolved, md := resolver.ReconcileProperties(ctxt, []string{"test-app"}, []string{"production", "mine"}, InjectedProperties{}, &source)

	assert.Equal(t, "backend-mine.yml > backend.yml", md.PrecedenceDisplayMessage)
	assert.Equal(t, []duplicate{{key: "owner", value: "Mine", source: "backend-mine.yml"}}, resolver.pointlessOverrides)
	assert.Equal(t, ResolvedConfigValues{"owner": "Mine", "type": "backend"}, resolved)
}

func Test_reconcileProperties_defaultValue(t *testing.T) {
	ctxt := context.Background()

	source := Source{Name: "xxx",
		PropertySources: []PropertySource{
			{Name: "application.yml", Source: map[string]interface{}{"glint.a": "b", "myService.url": "https://${MISSING:goodDefault}.glintpay.com"}},
		}}

	resolver := Resolver{}
	resolved, _ := resolver.ReconcileProperties(ctxt, []string{"xxx"}, []string{}, nil, &source)

	assert.Equal(t, resolved, ResolvedConfigValues{"glint.a": "b", "myService.url": "https://goodDefault.glintpay.com"})
	assert.Empty(t, resolver.pointlessOverrides)
}

func Test_reconcileProperties_missingPropertyRef(t *testing.T) {
	ctxt := context.Background()

	source := Source{Name: "xxx",
		PropertySources: []PropertySource{
			{Name: "application.yml", Source: map[string]interface{}{"glint.a": "b", "myService.url": "https://${NON_EXISTENT}.glintpay.com"}},
		}}

	resolver := Resolver{}
	resolved, _ := resolver.ReconcileProperties(ctxt, []string{"xxx"}, []string{}, nil, &source)
	assert.Equal(t, resolved, ResolvedConfigValues{"glint.a": "b", "myService.url": "https://.glintpay.com"})

	assert.Empty(t, resolver.pointlessOverrides)

	// Was: assert.PanicsWithValue(t, "Missing value for property [NON_EXISTENT]", func() { resolver.ReconcileProperties([]string{"xxx"}, []string{}, nil, &source) })
}

func Test_reconcileProperties_missingPlaceholder(t *testing.T) {
	ctxt := context.Background()

	source := Source{Name: "xxx",
		PropertySources: []PropertySource{
			{Name: "application.yml", Source: map[string]interface{}{"glint.a": "b", "myService.url": "https://${  }.glintpay.com"}},
		}}

	resolver := Resolver{}
	resolved, _ := resolver.ReconcileProperties(ctxt, []string{"xxx"}, []string{}, nil, &source)
	assert.Equal(t, resolved, ResolvedConfigValues{"glint.a": "b", "myService.url": "https://.glintpay.com"})

	assert.Empty(t, resolver.pointlessOverrides)

	// Was: assert.PanicsWithValue(t, "Missing placeholder [${  }] for property [myService.url]", func() { resolver.ReconcileProperties([]string{"xxx"}, []string{}, nil, &source) })
}

func Test_MapOverrideDoesntFailWithUncomparableTypesPanic(t *testing.T) {
	// Prepare expected data

	ctxt := context.Background()

	source := Source{Name: "xxx",
		PropertySources: []PropertySource{
			{Name: "application-mine.yml", Source: map[string]interface{}{"owner": map[string]interface{}{"a": "xxx"}}},
			{Name: "application.yml", Source: map[string]interface{}{"owner": map[string]interface{}{"a": "c"}, "type": "backend"}},
		}}

	resolver := Resolver{}

	var hierConfig Blah
	resolved, _ := resolver.ReconcileProperties(ctxt, []string{"xxx"}, []string{"production", "mine"}, nil, &source)
	err := test.MarshalHierarchicalTo(resolved, &hierConfig)
	assert.NoError(t, err)

	assert.Equal(t, hierConfig, Blah{Owner: map[string]interface{}{"a": "xxx"}})
	assert.Empty(t, resolver.pointlessOverrides)
}

func Test_MarshalHierarchicalTo(t *testing.T) {
	ctxt := context.Background()

	source := Source{Name: "test-app",
		PropertySources: []PropertySource{
			{Name: "backend-mine.yml", Source: map[string]interface{}{"owner": "Mine"}},
			{Name: "application-production.yml", Source: map[string]interface{}{"glint.name": "Production", "myService.host": "production"}},
			{Name: "backend.yml", Source: map[string]interface{}{"owner": "Unknown", "type": "backend"}},
			{Name: "application.yml", Source: map[string]interface{}{"glint.a": "b", "glint.b": "c", "glint.c": "d", "glint.name": "Default", "myService.host": "default", "myService.url": "https://${myService.host:UNUSED}.glintpay.com", "x.y.z": 123}},
		}}

	resolver := Resolver{}

	var hierConfig TestHierarchicalConfig
	resolved, _ := resolver.ReconcileProperties(ctxt, []string{"test-app"}, []string{"production", "mine"}, nil, &source)
	err := test.MarshalHierarchicalTo(resolved, &hierConfig)
	assert.NoError(t, err)

	assert.Equal(t, hierConfig, TestHierarchicalConfig{Glint: Glint{A: "b", B: "c", C: "d", Name: "Production"}, MyService: MyService{Host: "production", Url: "https://production.glintpay.com"}, Owner: "Mine", Type: "backend", X: X{Y: Y{Z: 123}}})
}

func Test_MarshalFlattenedTo(t *testing.T) {
	ctxt := context.Background()

	source := Source{Name: "test-app",
		PropertySources: []PropertySource{
			{Name: "backend-mine.yml", Source: map[string]interface{}{"owner": "Mine"}},
			{Name: "application-production.yml", Source: map[string]interface{}{"g.name": "Production", "myService.host": "production"}},
			{Name: "backend.yml", Source: map[string]interface{}{"owner": "Unknown", "type": "backend"}},
			{Name: "application.yml", Source: map[string]interface{}{"g.a": "b", "g.b": "c", "g.c": "d", "g.name": "Default", "myService.host": "default", "myService.url": "https://${myService.host:UNUSED}.glintpay.com", "x.y.z": 123}},
		}}

	resolver := Resolver{}

	var flattenedConfig TestFlattenedConfig
	resolved, _ := resolver.ReconcileProperties(ctxt, []string{"test-app"}, []string{"production", "mine"}, nil, &source)
	err := test.MarshalFlattenedTo(resolved, &flattenedConfig)
	assert.NoError(t, err)

	assert.Equal(t, flattenedConfig, TestFlattenedConfig{A: "b", B: "c", C: "d", Name: "Production", Host: "production", Url: "https://production.glintpay.com", Owner: "Mine", Type: "backend", Num: 123})
}

func Test_emptyPropertySource(t *testing.T) {
	ctxt := context.Background()

	source := Source{Name: "test-app",
		PropertySources: []PropertySource{}}

	resolver := Resolver{}
	resolved, md := resolver.ReconcileProperties(ctxt, []string{"test-app"}, []string{"production", "mine"}, InjectedProperties{}, &source)

	assert.Empty(t, md.PrecedenceDisplayMessage)
	assert.Empty(t, resolver.pointlessOverrides)
	assert.Empty(t, resolved)
}

type Blah struct {
	Owner map[string]interface{}
}

type CloudConfigInjected struct {
	ServiceName            string
	ServiceNameUnderscores string
}

type TestHierarchicalConfig struct {
	CloudConfig CloudConfigInjected
	Glint
	MyService
	Owner string
	Type  string
	X
}

type X struct{ Y }
type Y struct{ Z int16 }

type Glint struct {
	A    string
	B    string
	C    string
	Name string
}

type MyService struct {
	Host string
	Url  string
}

type TestFlattenedConfig struct {
	A     string `from:"g.a"`
	B     string `from:"g.b"`
	C     string `from:"g.c"`
	Name  string `from:"g.name"`
	Host  string `from:"myService.host"`
	Url   string `from:"myService.url"`
	Owner string
	Type  string
	Num   int16 `from:"x.y.z"`
}
